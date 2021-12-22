import os
from werkzeug.datastructures import MultiDict
from fastapi.encoders import jsonable_encoder
from .types import Image,PartialImage,Tags

def db_to_json(images, tag_mod=False):
    if tag_mod:
        tagmapping = []
        for im in images:
            im = jsonable_encoder(im)
            tagmapping.append(
                (
                    Tags(
                        im.pop("id"),
                        im.pop("name"),
                        im.pop("is_nsfw"),
                        im.pop("description"),
                    ),
                    im,
                )
            )
        tagmapping = MultiDict(tagmapping)
        tags_ = []
        for tag in tagmapping.keys():
            tag_images = tagmapping.getlist(tag.tag_id)
            tag_images = [
                dict(t, **{"url": "https://cdn.waifu.im/" + t["file"] + t["extension"]})
                for t in tag_images
            ]
            tags_.append(dict(vars(tag), **{"images": tag_images}))
        return jsonable_encoder(tags_)
    else:
        imagemapping = []
        for image in images:
            image = jsonable_encoder(image)
            tag = Tags(
                image.pop("id"),
                image.pop("name"),
                image.pop("is_nsfw"),
                image.pop("description"),
            )
            imagemapping.append((Image(**image), tag))
        imagemapping = MultiDict(imagemapping)
        images_list = []
        for im in imagemapping.keys():
            tags = imagemapping.getlist(im.image_id)
            images_list.append(dict(vars(im), **{"tags": tags}))
        return jsonable_encoder(images_list)


def is_sql_injection(string):
    for char in string:
        if not char.isalnum() and not char == ".":
            return True
    return False


def format_to_image(string):
    if not string:
        return []
    cleaned_images = []
    for potential_im in string.lower().split(","):
        if is_sql_injection(potential_im) or not potential_im:
            continue
        cleaned_images.append(PartialImage(*os.path.splitext(potential_im)))
    return cleaned_images


async def myendpoints(app, over18=None):
    rt = await app.state.pool.fetch("SELECT * FROM Tags")
    if over18 is None:
        return {
            "sfw": [tag["name"] for tag in rt if not tag["is_nsfw"]],
            "nsfw": [tag["name"] for tag in rt if tag["is_nsfw"]],
        }
    elif over18:
        return [tag["name"] for tag in rt if tag["is_nsfw"]]
    else:
        return [tag["name"] for tag in rt if not tag["is_nsfw"]]


async def myendpoints_info(app, over18=None):
    rt = await app.state.pool.fetch("SELECT * FROM Tags")
    if over18 is None:
        return {
            "sfw": [tag for tag in rt if not tag["is_nsfw"]],
            "nsfw": [tag for tag in rt if tag["is_nsfw"]],
        }
    elif over18:
        return [tag for tag in rt if tag["is_nsfw"]]
    else:
        return [tag for tag in rt if not tag["is_nsfw"]]


"""Determine if an image is already or not in the User gallery for the toggle url param"""


async def wich_action(image, insert, delete, user_id, conn):
    if not image:
        return
    for im in image:
        rt = await conn.fetchrow(
            "SELECT image FROM FavImages WHERE user_id=$1 and image=$2",
            user_id,
            im.file,
        )
        if rt:
            delete.append(im)
        else:
            insert.append(im)


"""Utils to format into apropriate sql query"""


def create_query(user_id, insert=None, delete=None):
    if insert:
        args = [(user_id, im.filen) for im in insert]
        return (
            "INSERT INTO FavImages(user_id,image) VALUES($1,$2) ON CONFLICT (user_id,image) DO NOTHING",
            args,
        )
    elif delete:
        args = [(user_id, im.file) for im in delete]
        return "DELETE FROM FavImages WHERE user_id=$1 and image=$2", args
