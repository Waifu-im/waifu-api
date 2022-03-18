from fastapi import FastAPI, HTTPException, Depends, Header, Request
from starlette.responses import Response
from itsdangerous import URLSafeSerializer, BadSignature
import subprocess
import json
import math
import asyncio

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


def APIblacklist(IP, reason):
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
    loop.run_in_executor(None, APIblacklist, request.client.host, "Automatic blacklist")
    raise HTTPException(
        status_code=429,
        detail="Too Many Requests, You have been added to the blacklist.",
    )


async def decode_token(
        s_key,
        authorization: str = Header(None),
):
    try:
        token = authorization.split(" ")[1]
        rule = URLSafeSerializer(s_key)
        info = rule.loads(token)
        return info
    except (TypeError, KeyError, AttributeError, IndexError, BadSignature):
        raise HTTPException(
            status_code=403,
            detail=f"Invalid Token, please check that you did correctly format it in the Authorization header and "
                   f"that the token is up to date.",
        )


async def is_valid_token(token_user_id, secret, connection):
    return bool(
        await connection.fetchrow(
            "SELECT id from Registered_user WHERE id=$1 and secret=$2 ",
            token_user_id,
            secret,
        )
    )


async def has_permissions(
        token_user_id,
        secret,
        permissions,
        connection,
):
    authorized = True
    for perm_name in permissions:
        has_perm = await connection.fetchrow(
            """
SELECT * from user_permissions
JOIN permissions ON permissions.name=user_permissions.permission
JOIN registered_user on registered_user.id=user_permissions.user_id
WHERE registered_user.id=$1 and registered_user.secret=$2 and (permissions.name=$3 or permissions.position > (SELECT position from permissions where name=$3) or permissions.name='admin')""",
            token_user_id,
            secret,
            perm_name,
        )
        if not has_perm:
            authorized = False
            break
    return authorized


async def check_permissions(self, *, request, permissions, check_identity_only=False, user_id=None):
    permissions = (permissions if isinstance(permissions, (list, tuple)) else (permissions,))
    connection = await request.app.state.pool.acquire()
    authorization = request.headers.get('authorization')
    if not authorization:
        raise HTTPException(
            status_code=401,
            detail="No Token, please check that you provided a Token and that your correctly formatted it in the "
                   "Authorization header.",
        )
    info = await decode_token(request.app.state.secret_key, authorization)
    if not check_identity_only or user_id:
        allowed_user = await has_permissions(
            info["id"], info["secret"], self.permissions, self.connection
        )
    else:
        allowed_user = await is_valid_token(
            info['id'], info['secret'], connection
        )
    await request.app.state.pool.release(connection)
    if allowed_user:
        request.state.user_id = info['id']
        return info
    raise HTTPException(
        status_code=403,
        detail="Invalid Token, You do not have the permissions to request this route please check that the token "
               "is up "
               "to date"
               f"{' and, as you requested the user_id query string that you have the permissions to do so' if user_id else ''}.",
    )