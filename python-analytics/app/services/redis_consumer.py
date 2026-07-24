import asyncio
import logging

import httpx
import redis.asyncio as redis
import redis.exceptions as redis_exceptions

from app.core.config import settings
from app.services.clickhouse_client import insert_event

logger = logging.getLogger("redis_consumer")


async def _ensure_group(client: redis.Redis) -> None:
    try:
        await client.xgroup_create(
            name=settings.redis_stream,
            groupname=settings.redis_consumer_group,
            id="0",
            mkstream=True,
        )
    except redis_exceptions.ResponseError as exc:
        if "BUSYGROUP" not in str(exc):
            raise


async def run_consumer(stop_event: asyncio.Event) -> None:
    """Reads events published by the Go core (tinder-core/internal/events)
    off a Redis Stream consumer group and persists them into ClickHouse.
    Runs as a background task for the lifetime of the FastAPI app."""
    redis_client = redis.from_url(settings.redis_url, decode_responses=True)
    await _ensure_group(redis_client)

    async with httpx.AsyncClient(timeout=5.0) as http_client:
        while not stop_event.is_set():
            try:
                response = await redis_client.xreadgroup(
                    groupname=settings.redis_consumer_group,
                    consumername=settings.redis_consumer_name,
                    streams={settings.redis_stream: ">"},
                    count=100,
                    block=2000,
                )
            except redis_exceptions.ConnectionError:
                logger.exception("redis connection lost, retrying")
                await asyncio.sleep(1)
                continue

            if not response:
                continue

            for _stream_name, messages in response:
                for message_id, fields in messages:
                    try:
                        await insert_event(http_client, fields)
                    except Exception:
                        logger.exception("failed to persist event %s", message_id)
                        continue
                    await redis_client.xack(settings.redis_stream, settings.redis_consumer_group, message_id)

    await redis_client.aclose()
