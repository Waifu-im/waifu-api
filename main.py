from typing import Optional
from fastapi import FastAPI,Depends,Request
from starlette.background import BackgroundTask
from starlette.exceptions import HTTPException as StarletteHTTPException
from fastapi.responses import JSONResponse
import uvicorn
from uvicorn.middleware.proxy_headers import ProxyHeadersMiddleware
import aiohttp
import typing

import json
from itsdangerous import URLSafeSerializer, BadSignature
import asyncpg

import traceback

from routers import public
from routers import registered

"""App initalization"""

app = FastAPI(docs_url=None, redoc_url=None)
app.include_router(public.router)
app.include_router(registered.router)
aiosession=None
pool=None

with open("json/credentials.json",'r') as f:
    dt=json.load(f)
    db_user=dt['db_user']
    db_password=dt['db_password']
    db_ip=dt['db_ip']
    db_name=dt['db_name']
    secret_key=dt['secret_key']

@app.on_event("startup")
async def create_session():
    app.state.secret_key=secret_key
    app.state.httpsession=aiohttp.ClientSession()
    app.state.pool = await asyncpg.create_pool(user=db_user,password=db_password,host=db_ip,database=db_name,min_size=5,max_size=5)

@app.on_event("shutdown")
async def close_session():
    await app.state.pool.close()
    await app.state.httpsession.close()

"""Global error handlers"""
@app.exception_handler(StarletteHTTPException)
async def http_exception_handler(request, e):
    return JSONResponse(status_code=e.status_code,content=dict(code=e.status_code,message=e.detail))

@app.exception_handler(Exception)
async def handle_exception(request:Request,e):
    return await http_exception_handler(request,StarletteHTTPException(status_code=500,detail="Sorry i couldn't process your request correctly, seems like there is a problem in the application."))

"""Log request for statistic purpose"""

@app.middleware("http")
async def add_logs(request: Request,call_next):
    response = await call_next(request)
    if response.status_code==200:
        response.background = BackgroundTask(log_request, request)
    return response

async def log_request(request):
    await request.app.state.pool.execute("INSERT INTO api_logs(remote_address,url,user_agent,user_id) VALUES($1,$2,$3,$4)",request.client.host,str(request.url),request.headers['User-Agent'],getattr(request.state,"user_id",None))

