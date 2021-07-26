import asyncio
import aiohttp
from aiohttp import web
from middlewares import setup_middlewares
import asyncio
import aiomysql
from discord.ext import tasks
from handlers import Web
from app_credentials import db_name,db_user,db_ip,db_password


app = web.Application()

@tasks.loop(minutes=30)
async def get_db():
    loop=asyncio.get_event_loop()
    if app.pool:
        await app.pool.clear()
    else:
        app.pool = await aiomysql.create_pool(user=db_user,password=db_password,host=db_ip,db=db_name,connect_timeout=10,loop=loop,autocommit=True)

def main():
    app.pool=None
    get_db.start()
    web_handlers = Web()
    web_handlers.configure(app)
    setup_middlewares(app)
    web.run_app(app,port=8034)
    print('Server started at http://0.0.0.0:8034')


if __name__ == '__main__':
    main()
