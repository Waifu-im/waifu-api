from fastapi import HTTPException


async def get_user_info(session, user_id):
    """get user information using discord bot ipc server"""
    resp = await session.get(f"http://127.0.0.1:8033/userinfos/?id={user_id}")
    if resp.status == 404:
        raise HTTPException(status_code=400, detail="Please provide a valid user_id")
    if resp.status != 200:
        raise HTTPException(
            status_code=500,
            detail="Sorry, something went wrong with the ipc request.",
        )
    return await resp.json()
