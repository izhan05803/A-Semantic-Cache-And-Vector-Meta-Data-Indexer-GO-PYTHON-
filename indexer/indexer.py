import chromadb
from sentence_transformers import SentenceTransformer
import uuid

class Indexer:
    def __init__(self):
        self.model=SentenceTransformer('all-MiniLM-L6-v2')
        self.client = chromadb.PersistentClient(path="data/chroma_db")
        self.collection = self.client.get_or_create_collection(name="semantic_cache")

    def add_to_cache(self, prompt: str, response: str, metadata: dict):
        embedding = self.model.encode(prompt).tolist()
        entry_id = str(uuid.uuid4())
        self.collection.add(
            documents=[prompt],
            metadatas=[{**metadata, "response": response}],
            ids=[entry_id],
            embeddings=[embedding]
        )

    def search_cache(self, prompt: str, threshold: float):
        embedding = self.model.encode(prompt).tolist()

        # Query ChromaDB for the closest match
        results = self.collection.query(
            query_embeddings=[embedding],
            n_results=1
        )

        # Check if we have results
        if results['distances'] and results['distances'][0]:
            # ChromaDB uses L2 distance by default (0 is perfect match, higher is less similar)
            # 1 - distance as a similarity approximation
            distance = results['distances'][0][0]
            similarity = 1 - distance

            if similarity >= threshold:
                return results['metadatas'][0][0]['response']

        return None


