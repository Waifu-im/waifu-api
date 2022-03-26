import asyncpg
import aioredis
import aiohttp
import pydantic

from fastapi import FastAPI, Depends, Request, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.openapi.docs import get_swagger_ui_html
from fastapi.openapi.utils import get_openapi
from fastapi.encoders import jsonable_encoder
from starlette.background import BackgroundTask
from starlette.exceptions import HTTPException as StarletteHTTPException
from fastapi.responses import JSONResponse
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

app = FastAPI(
    description="An easy to use api that allows you to get waifu pictures from an archive "
                "of over 4000 images and multiple tags!",
    title="waifu.im",
    version="2.0",
    docs_url=None,
    redoc_url=None,

)
app.add_middleware(
    CORSMiddleware,
    allow_origins=['*'],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


def custom_openapi_schema():
    if app.openapi_schema:
        return app.openapi_schema
    schema = get_openapi(
        title="waifu.im",
        version="2.0.0",
        description="An easy to use api that allows you to get waifu pictures from an archive "
                    "of over 4000 images and multiple tags!",
        routes=app.routes,
    )
    paths = list(schema["paths"])
    for route in paths:
        if f'{route}/' in paths:
            del schema["paths"][route]
    app.openapi_schema = schema
    return app.openapi_schema


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


@app.on_event("startup")
async def startup():
    await create_session()


@app.on_event("shutdown")
async def close_session():
    await app.state.pool.close()
    await app.state.httpsession.close()


@app.get("/docs", include_in_schema=False)
async def custom_swagger_ui_html():
    return get_swagger_ui_html(
        openapi_url=app.openapi_url,
        title=app.title + "- Interactive Documentation",
        swagger_favicon_url="/favicon.ico"
    )


"""Global error handlers"""


@app.exception_handler(StarletteHTTPException)
async def http_exception_handler(request, e):
    return JSONResponse(
        status_code=e.status_code, content=dict(code=e.status_code, detail=e.detail)
    )


@app.exception_handler(pydantic.error_wrappers.ValidationError)
async def custom_validation_exception_handler(request: Request, exc: pydantic.error_wrappers.ValidationError):
    return JSONResponse(
        status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
        content=jsonable_encoder({"detail": exc.errors()}),
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
    print("RECEIVING A REQUEST")
    response = await call_next(request)
    if response.status_code == 200:
        response.background = BackgroundTask(log_request, request)
    return response


async def log_request(request):
    await request.app.state.pool.execute(
        "INSERT INTO api_logs(remote_address,url,user_agent,user_id) VALUES($1,$2,$3,$4)",
        request.client.host,
        str(request.url),
        request.headers.get("user-agent"),
        getattr(request.state, "user_id", None),
    )
app.include_router(public.router)
app.include_router(registered.router)
app.openapi = custom_openapi_schema

