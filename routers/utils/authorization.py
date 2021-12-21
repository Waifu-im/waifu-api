from fastapi import FastAPI, HTTPException, Depends, Header, Request
from starlette.responses import Response
from itsdangerous import URLSafeSerializer, BadSignature
import subprocess
import json
import math
import asyncio

"""Get database credentials and ratelimit"""

with open("private/json/credentials.json", "r") as f:
    dt = json.load(f)
    db_user = dt["db_user"]
    db_password = dt["db_password"]
    db_ip = dt["db_ip"]
    db_name = dt["db_name"]
    secret_key = dt["secret_key"]
    timesrate = dt["timesrate"]
    perrate = dt["perrate"]


"""Write new ip to nginx configuration"""


def APIblacklist(IP, reason):
    with open("/etc/nginx/blacklist/api.conf", "r") as f:
        lines = f.readlines()
        inlines = False
        for i, line in enumerate(lines):
            if IP in line:
                inlines = True
        if not inlines:
            lines.append(f"deny {IP};{'#'+reason if reason else ''}")
    with open("/etc/nginx/blacklist/api.conf", "w") as f:
        f.write("\n".join(lines))
    process = subprocess.Popen(
        ["sudo", "service", "nginx", "reload"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    process.wait()
    return inlines


"""Callback and ip identifier for ratelimit"""


async def default_identifier(request: Request):
    path = request.scope["path"]
    if not path.endswith("/"):
        path += "/"
    return request.client.host + ":" + path


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
    loop = asyncio.get_event_loop()
    loop.run_in_executor(None, APIblacklist, request.client.host, "Automatic blacklist")
    raise HTTPException(
        status_code=429,
        detail="Too Many Requests, You have been added to the blacklist.",
    )


"""Token verification"""


async def is_valid_token(
    request: Request, authorization: str = Header(None), request_perms: int = None
):
    if not authorization:
        raise HTTPException(
            status_code=401,
            detail="No Token, please check that you provided a Token and that your correctly formated it in the Authorization header.",
        )
    try:
        token = authorization.split(" ")[1]
        rule = URLSafeSerializer(request.app.state.secret_key)
        info = rule.loads(token)
        user_secret = info.get("secret")
        user_id = int(info.get("id"))
    except (TypeError, KeyError, AttributeError, IndexError, BadSignature):
        raise HTTPException(
            status_code=403,
            detail=f"Invalid Token, please check that you did correctly format it in the Authorization header and that the token is up to date.",
        )

    else:
        if request_perms:
            perm_name = "manage_galleries"
            authorized = await request.app.state.pool.fetchrow(
                """SELECT * from user_permissions
JOIN permissions ON permissions.name=user_permissions.permission
JOIN registered_user on registered_user.id=user_permissions.user_id
WHERE registered_user.id=$1 and registered_user.secret=$2 and (permissions.name=$3 or permissions.position > (SELECT position from permissions where name=$3) or permissions.name='admin')""",
                user_id,
                user_secret,
                perm_name,
            )
        else:
            authorized = await request.app.state.pool.fetchrow(
                "SELECT id from Registered_user WHERE id=$1 and secret=$2 ",
                user_id,
                user_secret,
            )
        if authorized:
            request.state.user_id = user_id
            return info
        else:
            raise HTTPException(
                status_code=403,
                detail=f"Invalid Token, You do not have the permissions to request this route please check that the token is up to date{' and, as you requested the id url parameter that you have the permissions to do so' if request_perms else ''}.",
            )
