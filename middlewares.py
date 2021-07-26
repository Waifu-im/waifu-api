from aiohttp import web
import traceback
import sys

async def handle_404(request):
    data={'code':404,'error':f'nothing found in path: {request.path}'}
    return web.json_response(data,status=404)


async def handle_500(request,error):
    data={'code':500,'error':f"{type(error).__name__} : {error}"}
    traceback.print_exception(type(error), error, error.__traceback__, file=sys.stderr)
    return web.json_response(data,status=500)


def create_error_middleware(app,overrides):

    @web.middleware
    async def error_middleware(request, handler):
        try:
            return await handler(request)
        except web.HTTPException as ex:
            override = overrides.get(ex.status)
            if override:
                return await override(request)
            raise 
        except Exception as e:
          return await overrides[500](request,e)

    return error_middleware

def setup_middlewares(app):
    error_middleware = create_error_middleware(app,{
        404: handle_404,
        500: handle_500
    })
    app.middlewares.append(error_middleware)
    app.middlewares.append(web.normalize_path_middleware())