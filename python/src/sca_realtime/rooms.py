"""chat.chat_messages への送信・履歴取得。

ルームの一覧/作成/リネーム/名前解決はGo側(`sca room list/create/rename`)が担当するため、
ここには対話チャットセッション(chat.py)が必要とする最小限の関数だけを置く。
"""

from __future__ import annotations

from typing import List


def list_messages(client, room_id: str, limit: int = 30) -> List[dict]:
    res = (
        client.schema("chat")
        .table("chat_messages")
        .select("*")
        .eq("room_id", room_id)
        .order("created_at", desc=True)
        .limit(limit)
        .execute()
    )
    return list(reversed(res.data or []))


def send_message(client, room_id: str, sender_id: str, content: str) -> dict:
    res = (
        client.schema("chat")
        .table("chat_messages")
        .insert({"room_id": room_id, "sender_id": sender_id, "content": content})
        .execute()
    )
    if not res.data:
        raise SystemExit("メッセージの送信に失敗しました。")
    return res.data[0]
