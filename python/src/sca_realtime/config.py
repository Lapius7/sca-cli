"""設定ファイルの読み込み(Go側の`sca init`/`sca login`が書いたものをそのまま読む)。

パス・環境変数名はGo側(cmd/sca/config.go)と完全に一致させること:
- ディレクトリ: `$SCA_CONFIG_DIR`(未設定なら `~/.config/sca`)
- 設定: `config.env`(SUPABASE_URL / ANON_KEY / SERVICE_ROLE_KEY / EMAIL)
- セッション: `session.json`({"access_token","refresh_token","email"})
"""

from __future__ import annotations

import os
from pathlib import Path

CONFIG_DIR = Path(os.environ.get("SCA_CONFIG_DIR", str(Path.home() / ".config" / "sca")))
CONFIG_FILE = CONFIG_DIR / "config.env"
SESSION_FILE = CONFIG_DIR / "session.json"

DEFAULTS = {
    "SUPABASE_URL": "https://supabase.lapius7.com",
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
    for key in ("SUPABASE_URL", "ANON_KEY", "SERVICE_ROLE_KEY", "EMAIL"):
        if os.environ.get(key):
            values[key] = os.environ[key]
    return values
