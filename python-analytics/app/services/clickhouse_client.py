import json
from typing import Any

import httpx

from app.core.config import settings


def _to_row(fields: dict[str, Any]) -> dict[str, Any]:
    # The Go core is free to publish whatever payload shape a given ticket
    # needs (see events.Event.Payload) — we only pull out the two columns we
    # query on and keep the rest as opaque JSON.
    return {
        "type": fields.get("type", ""),
        "user_id": str(fields.get("user_id", "")),
        "payload": json.dumps(fields),
    }


async def insert_event(client: httpx.AsyncClient, event: dict[str, Any]) -> None:
    query = f"INSERT INTO {settings.clickhouse_db}.events FORMAT JSONEachRow"
    resp = await client.post(settings.clickhouse_url, params={"query": query}, content=json.dumps(_to_row(event)))
    resp.raise_for_status()


async def query_event_counts(client: httpx.AsyncClient, user_id: str) -> list[dict[str, Any]]:
    query = (
        f"SELECT type, count() AS cnt "
        f"FROM {settings.clickhouse_db}.events "
        f"WHERE user_id = {{user_id:String}} "
        f"GROUP BY type "
        f"FORMAT JSONEachRow"
    )
    resp = await client.get(
        settings.clickhouse_url,
        params={"query": query, "param_user_id": user_id},
    )
    resp.raise_for_status()
    lines = resp.text.strip().splitlines()
    return [json.loads(line) for line in lines if line]
