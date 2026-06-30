import chromadb
import uuid
from config import settings


class Indexer:
    def __init__(self, settings=settings):
        self.client = chromadb.PersistentClient(path=settings.CHROMA_DB_PATH)
        self.collection = self.client.get_or_create_collection(name="semantic_cache")

    def add_to_cache(self, prompt: str, response: str, metadata: dict):
        entry_id = str(uuid.uuid4())
        self.collection.add(
            documents=[prompt],
            metadatas=[{**metadata, "response": response}],
            ids=[entry_id],
        )

    def search_cache(self, prompt: str, threshold: float):
        results = self.collection.query(
            query_texts=[prompt],
            n_results=1
        )

        if results['distances'] and results['distances'][0]:
            distance = results['distances'][0][0]
            similarity = 1 - distance
            if similarity >= threshold:
                return results['metadatas'][0][0]['response']

        return None


