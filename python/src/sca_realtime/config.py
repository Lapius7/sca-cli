"""設定ファイルの読み込み(Go側の`sca init`/`sca login`が書いたものをそのまま読む)。

パス・環境変数名はGo側(cmd/sca/config.go)と完全一致させること:
- ディレクトリ: `$SCA_CONFIG_DIR`(未設定なら `~/.config/sca`)
- 設定: `config.env`(SUPABASE_URL / ANON_KEY / PYTHON_DIR)
- セッション: `session.json`({"access_token","refresh_token","email"})

既定のSUPABASE_URLはsupabase.lapius7.com自体ではなく、CLI専用のリバースプロキシ
(sandbox.lapius7.com/supabase-chat-app/api/)を指す。このプロキシがANON_KEYを付与して
中継するため、Python側もANON_KEYを一切持たない(空文字のまま送っても、プロキシ側で上書きされる)。
"""

from __future__ import annotations

import os
from pathlib import Path

CONFIG_DIR = Path(os.environ.get("SCA_CONFIG_DIR", str(Path.home() / ".config" / "sca")))
CONFIG_FILE = CONFIG_DIR / "config.env"
SESSION_FILE = CONFIG_DIR / "session.json"

DEFAULTS = {
    "SUPABASE_URL": "https://sandbox.lapius7.com/supabase-chat-app/api",
    # supabase-pyはsupabase_keyが空文字だと起動時にエラーになるため、意味のない
    # プレースホルダーを渡す(プロキシ側で実際のANON_KEYに必ず上書きされるので、
    # ここに何を書いても実際の認証には使われない)。
    "ANON_KEY": "sca-proxy-handles-this",
}


def load_config() -> dict:
    values = dict(DEFAULTS)
    if CONFIG_FILE.exists():
        for line in CONFIG_FILE.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip()
            if value:
                values[key] = value
    for key in ("SUPABASE_URL", "ANON_KEY", "PYTHON_DIR"):
        if os.environ.get(key):
            values[key] = os.environ[key]
    return values
