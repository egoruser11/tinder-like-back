import asyncio
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.routes import analytics, health
from app.services.redis_consumer import run_consumer


@asynccontextmanager
async def lifespan(app: FastAPI):
    stop_event = asyncio.Event()
    consumer_task = asyncio.create_task(run_consumer(stop_event))

    yield

    stop_event.set()
    await consumer_task


app = FastAPI(title="tinder-analytics", lifespan=lifespan)
app.include_router(health.router)
app.include_router(analytics.router)
