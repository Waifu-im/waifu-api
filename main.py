import traceback
import sys

import json
import asyncpg
import aioredis

from fastapi import FastAPI, Depends, Request
from starlette.background import BackgroundTask
from starlette.exceptions import HTTPException as StarletteHTTPException
from fastapi.responses import JSONResponse
import aiohttp
from fastapi_limiter import FastAPILimiter

from routers import public
from routers import registered
from routers.utils import (
    db_user,
    db_password,
    db_ip,
    db_name,
    secret_key,
    default_identifier,
    default_callback,
    MANY_LIMIT,
    ImageQueue,
)

app = FastAPI(docs_url=None, redoc_url=None)
app.include_router(public.router)
app.include_router(registered.router)


@app.on_event("startup")
async def create_session():
    app.state.secret_key = secret_key
    app.state.httpsession = aiohttp.ClientSession()
    app.state.pool = await asyncpg.create_pool(
        user=db_user,
        password=db_password,
        host=db_ip,
        database=db_name,
        min_size=5,
        max_size=5,
    )
    redis = await aioredis.from_url(
        "redis://localhost", encoding="utf-8", decode_responses=True
    )
    await FastAPILimiter.init(
        redis, identifier=default_identifier, callback=default_callback
    )
    app.state.last_images = ImageQueue(redis, "api_last_images", MANY_LIMIT)


@app.on_event("shutdown")
async def close_session():
    await app.state.pool.close()
    await app.state.httpsession.close()


"""Global error handlers"""


@app.exception_handler(StarletteHTTPException)
async def http_exception_handler(request, e):
    return JSONResponse(
        status_code=e.status_code, content=dict(code=e.status_code, message=e.detail)
    )


@app.exception_handler(Exception)
async def exception_handler(request, e):
    return await http_exception_handler(
        request,
        StarletteHTTPException(
            status_code=500,
            detail="Sorry i couldn't process your request correctly, seems like there is a problem in the application.",
        ),
    )


"""Log request for statistic purpose"""


@app.middleware("http")
async def add_logs(request: Request, call_next):
    try:
        response = await call_next(request)
    except Exception as error:
        traceback.print_exception(
            type(error), error, error.__traceback__, file=sys.stderr
        )
        return await http_exception_handler(
            request,
            StarletteHTTPException(
                status_code=500,
                detail="Sorry i couldn't process your request correctly, seems like there is a problem in the application.",
            ),
        )
    if response.status_code == 200:
        response.background = BackgroundTask(log_request, request)
    return response


async def log_request(request):
    await request.app.state.pool.execute(
        "INSERT INTO api_logs(remote_address,url,user_agent,user_id) VALUES($1,$2,$3,$4)",
        request.client.host,
        str(request.url),
        request.headers["User-Agent"],
        getattr(request.state, "user_id", None),
    )
