from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    INDEXER_PORT: int = 50051
    CHROMA_DB_PATH: str = "data/chroma_db"
    EMBEDDING_MODEL: str = "all-MiniLM-L6-v2"
    INDEXER_API_KEY: str = ""

    class Config:
        env_file = ".env"

# Create a global instance
settings = Settings()
