"""
UPS-ECO-SYSTEM · Analytics Service
Python + FastAPI — MRP, себестоимость, отчёты
"""

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

app = FastAPI(
    title="UPS Analytics",
    version="0.1.0",
    description="MRP, cost calculation, reports"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health")
async def health():
    return {"status": "ok", "service": "ups-analytics"}


@app.get("/api/v1/mrp/run")
async def run_mrp():
    """Заглушка ежедневного MRP (Этап 5)"""
    return {
        "status": "not_implemented",
        "message": "MRP calculation will be implemented in Stage 5"
    }


@app.get("/api/v1/cost/{project_id}")
async def project_cost(project_id: str):
    """Себестоимость проекта ≤ 10 сек (критерий приёмки №6)"""
    return {
        "project_id": project_id,
        "status": "not_implemented"
    }
