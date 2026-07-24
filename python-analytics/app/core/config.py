from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    redis_url: str = "redis://localhost:6379/0"
    redis_stream: str = "tinder:events"
    redis_consumer_group: str = "analytics"
    redis_consumer_name: str = "analytics-1"

    clickhouse_url: str = "http://localhost:8123"
    clickhouse_db: str = "analytics"

    class Config:
        env_file = ".env"


settings = Settings()
