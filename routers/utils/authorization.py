import asyncio
import json
import math
import subprocess

from fastapi import HTTPException, Request
from itsdangerous import URLSafeSerializer, BadSignature
from starlette.responses import Response

INVALID_TOKEN_MESSAGE = f"Invalid Token, please check that you did correctly format it in the Authorization header " \
                        f"and that the token is up to date. "
NO_PERMISSIONS_MESSAGE = "You do not have the permissions to access this content."
NOT_AUTHENTICATED_MESSAGE = "Not authenticated"

with open("private/json/credentials.json", "r") as f:
    """Get database credentials and ratelimit"""
    dt = json.load(f)
    db_user = dt["db_user"]
    db_password = dt["db_password"]
    db_ip = dt["db_ip"]
    db_name = dt["db_name"]
    secret_key = dt["secret_key"]
    timesrate = dt["timesrate"]
    perrate = dt["perrate"]


def api_blacklist(IP, reason):
    """Write new ip to nginx configuration"""
    with open("/etc/nginx/blacklist/api.conf", "r") as f:
        lines = f.readlines()
        inlines = False
        for i, line in enumerate(lines):
            if IP in line:
                inlines = True
        if not inlines:
            lines.append(f"deny {IP};{'#' + reason if reason else ''}")
    with open("/etc/nginx/blacklist/api.conf", "w") as f:
        f.write("\n".join(lines))
    process = subprocess.Popen(
        ["sudo", "service", "nginx", "reload"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    process.wait()
    return inlines


def reformat_token(token):
    if not token:
        raise HTTPException(
            status_code=401,
            detail=NOT_AUTHENTICATED_MESSAGE,
        )
    if "bearer" in token.lower():
        token = token.replace("Bearer", "").replace(" ", "")
    return token


async def default_identifier(request: Request):
    """IP identifier for ratelimit"""
    return request.client.host + ":" + str(request.url)


async def default_callback(request: Request, response: Response, pexpire: int):
    """
    default callback when too many requests
    :param request:
    :param pexpire: The remaining milliseconds
    :param response:
    :return:
    """
    expire = math.ceil(pexpire / 1000)
    raise HTTPException(
        status_code=429,
        detail="You are being ratelimited, please check the Retry-After header.",
        headers={"Retry-After": str(expire)},
    )


async def blacklist_callback(request: Request, response: Response, pexpire: int):
    """Auto-blacklist callback for ratelimit"""
    loop = asyncio.get_event_loop()
    loop.run_in_executor(None, api_blacklist, request.client.host, "Automatic blacklist")
    raise HTTPException(
        status_code=429,
        detail="Too Many Requests, You have been added to the blacklist.",
    )


async def decode_token(
        s_key,
        token,
):
    try:
        rule = URLSafeSerializer(s_key)
        info = rule.loads(token)
        return info
    except (TypeError, KeyError, AttributeError, IndexError, BadSignature):
        raise HTTPException(
            status_code=403,
            detail=INVALID_TOKEN_MESSAGE,
        )


async def is_valid_credentials(token_user_id, secret, connection):
    return bool(
        await connection.fetchrow(
            "SELECT id from Registered_user WHERE id=$1 and secret=$2 ",
            token_user_id,
            secret,
        )
    )


async def has_permissions(
        user_id,
        permissions,
        connection,
        target_id=None,
):
    if target_id and user_id == target_id:
        return True
    authorized = True
    for perm_name in permissions:
        has_perm = await connection.fetchrow(
            "SELECT * from user_permissions "
            "JOIN permissions ON permissions.name=user_permissions.permission "
            "JOIN registered_user on registered_user.id=user_permissions.user_id "
            "WHERE registered_user.id=$1 "
            "and (permissions.name=$2 or permissions.position > (SELECT position from permissions where name=$2) "
            "or permissions.name='admin' ) and (target_id=$3 or target_id=$4)",
            user_id,
            perm_name,
            target_id,
            None,
        )
        if not has_perm:
            authorized = False
            break
    return authorized


async def check_user_permissions(*, request, permissions, user_id, target_id=None):
    if not await has_permissions(user_id, permissions, request.app.state.pool, target_id=target_id):
        raise HTTPException(
            status_code=403,
            detail=NO_PERMISSIONS_MESSAGE,
        )
    return True


async def get_token_info(*, request, token):
    token = reformat_token(token)
    info = await decode_token(request.app.state.secret_key, token)
    if not await is_valid_credentials(info['id'], info['secret'], request.app.state.pool):
        raise HTTPException(
            status_code=401,
            detail=INVALID_TOKEN_MESSAGE,
        )
    return info
