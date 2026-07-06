"""preprint-af — turns a research folder into a submission-ready scientific paper."""

from __future__ import annotations

import os

from agentfield import Agent, AIConfig, HarnessConfig

from preprint_af.reasoners import (
    blueprint_router,
    build_router,
    critique_router,
    intake_router,
    latex_router,
    positioning_router,
    prose_router,
    repair_router,
    workflow_router,
)

DEFAULT_MODEL = "openrouter/deepseek/deepseek-v4-pro"


def build_app() -> Agent:
    app = Agent(
        node_id=os.getenv("AGENT_NODE_ID", "preprint-af"),
        agentfield_server=os.getenv("AGENTFIELD_SERVER", "http://localhost:8080"),
        version="2.0.0",
        description=(
            "Turns a research folder into a submission-ready scientific paper: evidence ledger, "
            "positioning tournament, parallel section/figure drafting, compile gate, and a "
            "convergence-driven critique and repair loop."
        ),
        tags=["scientific-writing", "paper-generation", "latex", "figures"],
        ai_config=AIConfig(model=os.getenv("AI_MODEL", DEFAULT_MODEL)),
        harness_config=HarnessConfig(
            provider="opencode",
            model=os.getenv("OPENCODE_MODEL", DEFAULT_MODEL),
            max_turns=int(os.getenv("HARNESS_MAX_TURNS", "40")),
            max_budget_usd=float(os.getenv("HARNESS_MAX_BUDGET_USD", "5.00")),
            permission_mode=os.getenv("HARNESS_PERMISSION_MODE", "auto"),
        ),
        dev_mode=True,
    )
    for router in (
        intake_router,
        positioning_router,
        blueprint_router,
        build_router,
        latex_router,
        critique_router,
        prose_router,
        repair_router,
        workflow_router,
    ):
        app.include_router(router)
    return app
