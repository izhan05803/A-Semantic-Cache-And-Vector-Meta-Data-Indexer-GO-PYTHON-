import sys
import os
from concurrent import futures
import time

# Ensure this script's directory is on the path so local imports work
_script_dir = os.path.dirname(os.path.abspath(__file__))
if _script_dir not in sys.path:
    sys.path.insert(0, _script_dir)

import grpc
from proto_gen import cache_service_pb2
from proto_gen import cache_service_pb2_grpc
from indexer import Indexer

class CacheService(cache_service_pb2_grpc.CacheServiceServicer):
    def __init__(self):
        self.indexer = Indexer()

    def CheckCache(self, request, context):
        print(f"Received CheckCache request for prompt: {request.prompt}")
        response_content = self.indexer.search_cache(request.prompt, request.similarity_threshold)

        if response_content:
            return cache_service_pb2.CacheResponse(hit=True, response=response_content, score=1.0) # Placeholder score
        else:
            return cache_service_pb2.CacheResponse(hit=False, response="", score=0.0)

    def UpdateCache(self, request, context):
        print(f"Received UpdateCache request for prompt: {request.prompt}")
        self.indexer.add_to_cache(request.prompt, request.response, dict(request.metadata))
        return cache_service_pb2.UpdateResponse(success=True)

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    cache_service_pb2_grpc.add_CacheServiceServicer_to_server(CacheService(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    print("Indexer gRPC server started on port 50051")
    try:
        while True:
            time.sleep(86400) # One day in seconds
    except KeyboardInterrupt:
        server.stop(0)

if __name__ == '__main__':
    serve()
