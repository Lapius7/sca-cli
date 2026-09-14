"""オンライン一覧の一発確認(who)と、対話チャットセッション(join)。

realtime-py(supabase-pyの内部で使われるRealtimeクライアント)は非同期(asyncio)前提のため、
バックグラウンドスレッドに専用のイベントループを立てて接続を維持し、
メインスレッドは通常の`input()`ループでメッセージ送信を受け付ける構成にしている。
"""

from __future__ import annotations

import asyncio
import threading
from typing import List, Optional

from realtime import RealtimePostgresChangesListenEvent, RealtimeSubscribeStates
from supabase import acreate_client

from . import rooms
from .profiles import get_display_name, make_cache

_PROBE_KEY = "sca-who-probe"


async def _who_once_async(cfg: dict, session: dict, room_id: str, timeout: float) -> List[str]:
    client = await acreate_client(cfg["SUPABASE_URL"], cfg["ANON_KEY"])
    await client.auth.set_session(session["access_token"], session["refresh_token"])
    await client.realtime.set_auth(session["access_token"])

    got_sync = asyncio.Event()
    online_ids: List[str] = []

    channel = client.channel(f"room-{room_id}", {"config": {"presence": {"key": _PROBE_KEY}}})

    def on_sync():
        nonlocal online_ids
        online_ids = [k for k in channel.presence_state().keys() if k != _PROBE_KEY]
        got_sync.set()

    channel.on_presence_sync(on_sync)
    await channel.subscribe()

    try:
        await asyncio.wait_for(got_sync.wait(), timeout=timeout)
    except asyncio.TimeoutError:
        pass

    await client.remove_channel(channel)
    await client.realtime.close()
    return online_ids


def who_once(cfg: dict, session: dict, room_id: str, timeout: float = 4.0) -> List[str]:
    """一瞬だけRealtimeに接続し、Presenceの初回syncを待って現在のオンライン一覧を返す。"""
    return asyncio.run(_who_once_async(cfg, session, room_id, timeout))


class ChatSession:
    """バックグラウンドスレッドでRealtime接続(メッセージ受信+Presence)を保持するクラス。"""

    def __init__(self, cfg: dict, session: dict, room: dict, user_id: str, sync_client):
        self.cfg = cfg
        self.session = session
        self.room = room
        self.user_id = user_id
        self.sync_client = sync_client
        self.profile_cache = make_cache()
        self.online_ids: set = set()

        self._thread: Optional[threading.Thread] = None
        self._loop: Optional[asyncio.AbstractEventLoop] = None
        self._client = None
        self._channel = None
        self._ready = threading.Event()
        self._stop = threading.Event()

    def start(self) -> None:
        self._thread = threading.Thread(target=self._run_loop, daemon=True)
        self._thread.start()
        self._ready.wait(timeout=15)

    def _run_loop(self) -> None:
        self._loop = asyncio.new_event_loop()
        asyncio.set_event_loop(self._loop)
        self._loop.run_until_complete(self._async_main())

    async def _async_main(self) -> None:
        self._client = await acreate_client(self.cfg["SUPABASE_URL"], self.cfg["ANON_KEY"])
        await self._client.auth.set_session(
            self.session["access_token"], self.session["refresh_token"]
        )
        await self._client.realtime.set_auth(self.session["access_token"])

        room_id = self.room["id"]
        self._channel = self._client.channel(
            f"room-{room_id}", {"config": {"presence": {"key": self.user_id}}}
        )

        def on_message(payload: dict) -> None:
            record = (payload.get("data") or {}).get("record")
            if not record or record.get("sender_id") == self.user_id:
                return  # 自分の発言は送信時に既に表示済みなので無視
            name = get_display_name(self.sync_client, record["sender_id"], self.profile_cache)
            print(f"\n[{name}] {record.get('content', '')}\n> ", end="", flush=True)

        def on_sync() -> None:
            new_ids = set(self._channel.presence_state().keys())
            for uid in new_ids - self.online_ids:
                if uid == self.user_id:
                    continue
                name = get_display_name(self.sync_client, uid, self.profile_cache)
                print(f"\n* {name} さんが入室しました\n> ", end="", flush=True)
            for uid in self.online_ids - new_ids:
                if uid == self.user_id:
                    continue
                name = get_display_name(self.sync_client, uid, self.profile_cache)
                print(f"\n* {name} さんが退室しました\n> ", end="", flush=True)
            self.online_ids = new_ids

        self._channel.on_postgres_changes(
            RealtimePostgresChangesListenEvent.Insert,
            schema="chat",
            table="chat_messages",
            filter=f"room_id=eq.{room_id}",
            callback=on_message,
        )
        self._channel.on_presence_sync(on_sync)

        def on_subscribe(status, err) -> None:
            if status == RealtimeSubscribeStates.SUBSCRIBED:
                assert self._channel is not None
                asyncio.create_task(self._channel.track({"user_id": self.user_id}))
                self._ready.set()
            elif err:
                print(f"\n[接続エラー] {err}")
                self._ready.set()

        await self._channel.subscribe(on_subscribe)

        # stop()が呼ばれるまでこのループを生かしておく(コールバックはこのループ上で動く)
        while not self._stop.is_set():
            await asyncio.sleep(0.2)

        await self._channel.untrack()
        await self._client.remove_channel(self._channel)
        await self._client.realtime.close()

    def who(self) -> List[str]:
        return sorted(uid for uid in self.online_ids if uid != self.user_id)

    def stop(self) -> None:
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=5)


def run_interactive(cfg: dict, session: dict, room: dict, sync_client, user_id: str) -> None:
    print(f"=== {room['name']} に入室しました ===")
    print(
        "メッセージを入力してEnterで送信。"
        " /who でオンライン一覧、 /invite で招待方法、 /quit またはCtrl+C で退室。"
    )

    chat = ChatSession(cfg, session, room, user_id, sync_client)
    chat.start()

    try:
        while True:
            try:
                line = input("> ")
            except (EOFError, KeyboardInterrupt):
                print()
                break

            line = line.strip()
            if not line:
                continue
            if line in ("/quit", "/leave", "/exit"):
                break
            if line == "/who":
                online = chat.who()
                if not online:
                    print("(自分以外に誰もいません)")
                else:
                    names = [get_display_name(sync_client, uid, chat.profile_cache) for uid in online]
                    print("オンライン: " + ", ".join(names))
                continue

            if line == "/invite":
                invite_url = f"https://sandbox.lapius7.com/supabase-chat-app/{room['id']}"
                print(f"CLIから:     sca room join {room['name']}")
                print(f"ブラウザから: {invite_url}")
                continue

            rooms.send_message(sync_client, room["id"], user_id, line)
    finally:
        chat.stop()
        print("退室しました。")
