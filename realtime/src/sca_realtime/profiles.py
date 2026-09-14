"""user_id -> 表示名(public.user_profiles)の解決。"""

from __future__ import annotations

from typing import Dict


def make_cache() -> Dict[str, str]:
    return {}


def get_display_name(client, user_id: str, cache: Dict[str, str]) -> str:
    if user_id in cache:
        return cache[user_id]
    name = user_id[:8]
    try:
        res = (
            client.table("user_profiles")
            .select("display_name, handle")
            .eq("user_id", user_id)
            .maybe_single()
            .execute()
        )
        if res and res.data:
            name = res.data.get("display_name") or res.data.get("handle") or name
    except Exception:
        pass
    cache[user_id] = name
    return name
