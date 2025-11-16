#!/usr/bin/env python3
"""
Mock Inference Service

A simple FastAPI service that mocks OpenAI-compatible inference endpoints
for local development and testing.

Endpoints:
  - POST /v1/completions - Text completion endpoint
  - POST /v1/chat/completions - Chat completion endpoint
  - GET /health - Health check endpoint
  - GET /ready - Readiness check endpoint
"""

from fastapi import FastAPI, Request
from pydantic import BaseModel
import uvicorn
import asyncio

app = FastAPI(title="Mock Inference Service", version="1.0.0")


class CompletionRequest(BaseModel):
    """Request model for text completion endpoint."""
    prompt: str
    max_tokens: int = 100
    model: str = "gpt-4o"


class CompletionResponse(BaseModel):
    """Response model for text completion endpoint."""
    text: str
    tokens_used: int
    model: str


@app.post("/v1/completions")
async def completions(req: CompletionRequest, request: Request):
    """
    Mock text completion endpoint.
    
    Simulates a text completion by appending a mock response suffix
    and calculating token usage based on prompt length.
    """
    await asyncio.sleep(0.1)  # Simulate processing delay
    tokens = len(req.prompt.split()) + 10
    return CompletionResponse(
        text=req.prompt + " [mock inference response]",
        tokens_used=min(tokens, req.max_tokens),
        model=req.model,
    )


@app.post("/v1/chat/completions")
async def chat_completions(request: Request):
    """
    Mock chat completion endpoint.
    
    Returns a simple mock chat response with fixed token usage.
    """
    await asyncio.sleep(0.1)  # Simulate processing delay
    return {
        "choices": [
            {
                "message": {"role": "assistant", "content": "[mock chat response]"},
                "finish_reason": "stop",
            }
        ],
        "usage": {"total_tokens": 42},
    }


@app.get("/health")
async def health():
    """Health check endpoint."""
    return {"status": "healthy", "service": "mock-inference"}


@app.get("/ready")
async def ready():
    """Readiness check endpoint."""
    return {"ready": True}


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)

