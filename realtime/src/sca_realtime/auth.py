"""セッション(session.json)の読み込みと、認証済みsupabase-pyクライアントの生成。

ログイン(マジックリンク発行)自体はGo側(`sca login`)が担当し、session.jsonを書き込む。
この関数はGoが書いたものと同じJSON形式({"access_token","refresh_token","email"})を
そのまま読むだけ。
"""

from __future__ import annotations

from typing import Optional

from supabase import Client, create_client

from .config import SESSION_FILE, load_config


def load_session() -> Optional[dict]:
    if not SESSION_FILE.exists():
        return None
    import json

    return json.loads(SESSION_FILE.read_text(encoding="utf-8"))


def get_client() -> Client:
    cfg = load_config()
    session = load_session()
    if not session:
        raise SystemExit("ログインしていません。先に `sca login` を実行してください。")

    client = create_client(cfg["SUPABASE_URL"], cfg["ANON_KEY"])
    try:
        client.auth.set_session(session["access_token"], session["refresh_token"])
    except Exception as e:
        raise SystemExit(
            f"セッションの復元に失敗しました。`sca login` でログインし直してください: {e}"
        )
    return client


def get_user_id(client: Client) -> str:
    res = client.auth.get_user()
    if not res or not res.user:
        raise SystemExit("セッションが切れています。`sca login` でログインし直してください。")
    return res.user.id
