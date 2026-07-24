import httpx
from fastapi import APIRouter, Depends

from app.services.clickhouse_client import query_event_counts

router = APIRouter(prefix="/analytics")


async def get_http_client() -> httpx.AsyncClient:
    async with httpx.AsyncClient(timeout=5.0) as client:
        yield client


@router.get("/users/{user_id}/stats")
async def user_stats(user_id: str, client: httpx.AsyncClient = Depends(get_http_client)) -> dict:
    counts = await query_event_counts(client, user_id)
    return {"user_id": user_id, "events": counts}

# ticket-9: add /analytics/users/{id}/hot-score (rolling like-rate) — the
#           value tinder-core will fetch to rank the discovery feed.
# ticket-10: add /analytics/retention, /analytics/matches-per-day
