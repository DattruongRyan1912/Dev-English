"""Small, local-only embedding service for DevEnglish.

The application deliberately keeps this process separate from the Go binary:
the model can be upgraded or moved to a different node without changing the
canonical Knowledge contracts.  The endpoint accepts raw text and applies the
multilingual-e5 retrieval prefixes here, so callers never need to know model
specific prompt rules.
"""

from contextlib import asynccontextmanager
import os
import threading
from typing import Literal

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer


MODEL_NAME = os.getenv("EMBEDDING_MODEL", "intfloat/multilingual-e5-small")
MODEL_DEVICE = os.getenv("EMBEDDING_DEVICE", "cpu")
MODEL_DIMENSIONS = 384
MAX_TEXT_CHARS = int(os.getenv("EMBEDDING_MAX_TEXT_CHARS", "16000"))


class EmbedRequest(BaseModel):
    text: str = Field(min_length=1, max_length=MAX_TEXT_CHARS)
    input_type: Literal["query", "passage"] = "query"


class EmbedResponse(BaseModel):
    embedding: list[float]
    dimensions: int
    model: str


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Loading once at startup gives Docker a truthful readiness signal and
    # prevents the first user request from paying the model-load cost.
    app.state.encoder = SentenceTransformer(MODEL_NAME, device=MODEL_DEVICE)
    app.state.encode_lock = threading.Lock()
    yield


app = FastAPI(title="DevEnglish embedding sidecar", version="1.0", lifespan=lifespan)


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ready", "model": MODEL_NAME}


@app.post("/embed", response_model=EmbedResponse)
def embed(request: EmbedRequest) -> EmbedResponse:
    text = request.text.strip()
    if not text:
        raise HTTPException(status_code=400, detail="text is required")
    prefixed = f"{request.input_type}: {text}"
    try:
        with app.state.encode_lock:
            vector = app.state.encoder.encode(
                [prefixed],
                normalize_embeddings=True,
                convert_to_numpy=True,
            )[0].tolist()
    except Exception as exc:  # pragma: no cover - exercised through runtime health
        # Do not return model internals or request content in the error body.
        raise HTTPException(status_code=503, detail="embedding unavailable") from exc
    if len(vector) != MODEL_DIMENSIONS:
        raise HTTPException(status_code=503, detail="embedding dimensions unavailable")
    return EmbedResponse(
        embedding=vector,
        dimensions=MODEL_DIMENSIONS,
        model=MODEL_NAME,
    )
