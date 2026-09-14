"""`python3 -m sca_realtime <who|join> <room_id> <room_name>`

Go側の`sca`バイナリから内部的に呼び出されるヘルパー。ユーザーが直接使うことは想定していない
(ルームの名前解決・一覧・作成・リネームはGo側で完結する。ここではRealtime接続が必要な
「オンライン一覧」と「対話チャット」だけを担当する)。
"""

from __future__ import annotations

import sys

from . import auth, chat
from .config import load_config
from .profiles import get_display_name, make_cache


def main() -> None:
    if len(sys.argv) < 4:
        print("usage: python -m sca_realtime <who|join> <room_id> <room_name>", file=sys.stderr)
        sys.exit(2)

    action = sys.argv[1]
    room_id = sys.argv[2]
    room_name = sys.argv[3]

    cfg = load_config()
    session = auth.load_session()
    if not session:
        print("ログインしていません。`sca login` を実行してください。", file=sys.stderr)
        sys.exit(1)

    if action == "who":
        online_ids = chat.who_once(cfg, session, room_id)
        if not online_ids:
            print(f"{room_name} には現在誰もいません。")
            return
        client = auth.get_client()
        cache = make_cache()
        names = [get_display_name(client, uid, cache) for uid in online_ids]
        print(f"{room_name} にいる人: " + ", ".join(names))
        return

    if action == "join":
        client = auth.get_client()
        user_id = auth.get_user_id(client)
        room = {"id": room_id, "name": room_name}
        chat.run_interactive(cfg, session, room, client, user_id)
        return

    print(f"unknown action: {action}", file=sys.stderr)
    sys.exit(2)


if __name__ == "__main__":
    main()
