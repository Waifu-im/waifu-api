import os
from werkzeug.datastructures import MultiDict
from fastapi.encoders import jsonable_encoder
from .types import Image, PartialImage, Tags, OrderByType,CustomBool
from .constants import MANY_LIMIT


def format_gif(is_gif):
    if is_gif:
        return "Images.extension='.gif'"
    else:
        return "not Images.extension='.gif'"


def format_limit(many):
    return f"LIMIT {MANY_LIMIT if many else '1'}"


def format_order_by(order_by, table_prefix=None, disable_random=None):
    if order_by == OrderByType.favourite:
        return f"ORDER BY {table_prefix if table_prefix else ''}favourites DESC"
    if order_by == OrderByType.uploaded_at:
        return f"ORDER BY {table_prefix if table_prefix else ''}uploaded_at DESC"
    else:
        return "" if disable_random else " ORDER BY RANDOM()"


def format_tags_where(selected_tags, excluded_tags):
    results = []
    if selected_tags:
        results.append(f"Tags.name in ({format_in(selected_tags)})")
    if excluded_tags:
        results.append("NOT EXISTS"
                       "(SELECT 1 FROM LinkedTags AS lk JOIN Tags T ON lk.tag_id=T.id WHERE lk.image = Images.file "
                       f"AND T.name in ({format_in(excluded_tags)}))")
    return " and ".join(results)


def format_nsfw(is_nsfw):
    string = "Images.is_nsfw"
    return string if (is_nsfw == CustomBool.true or is_nsfw == CustomBool.null) else 'not ' + string


def format_image_type(is_nsfw, tags):
    if is_nsfw == CustomBool.null:
        return ''
    if tags:
        return f"and ({f'{format_nsfw(is_nsfw)} or ' if is_nsfw is not None else ''}" \
               f"EXISTS(SELECT name from Tags T2 WHERE T2.is_nsfw AND T2.name in ({format_in(tags)})))"
    return f'and {format_nsfw(is_nsfw)}'


def format_in(_list):
    return ','.join(["'" + i + "'" for i in _list])


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
                        im.pop("description"),
                        im.pop("tag_is_nsfw"),
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
                image.pop("description"),
                image.pop("tag_is_nsfw"),
            )
            imagemapping.append((Image(**image), tag))
        imagemapping = MultiDict(imagemapping)
        images_list = []
        for im in imagemapping.keys():
            tags = imagemapping.getlist(im.image_id)
            images_list.append(dict(vars(im), **{"tags": tags}))
        return jsonable_encoder(images_list)


def format_to_image(images_list):
    return [PartialImage(*os.path.splitext(im)) for im in images_list]


async def get_tags(app, full=False):
    raw_result = await app.state.pool.fetch("SELECT * FROM Tags")
    tag_list = []
    for item in raw_result:
        dict_item = dict(item)
        dict_item['tag_id'] = dict_item.pop('id')
        tag_list.append(dict_item)
    return {
        "versatile": [tag if full else tag["name"] for tag in tag_list if not tag["is_nsfw"]],
        "nsfw": [tag if full else tag["name"] for tag in tag_list if tag["is_nsfw"]]
    }


async def wich_action(image, insert, delete, user_id, conn):
    """Determine if an image is already or not in the User gallery for the toggle url param"""
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


def create_query(user_id, insert=None, delete=None):
    """Utils to format into appropriate sql query"""
    if insert:
        args = [(user_id, im.file) for im in insert]
        return (
            "INSERT INTO FavImages(user_id,image) VALUES($1,$2)",
            args,
        )
    elif delete:
        args = [(user_id, im.file) for im in delete]
        return "DELETE FROM FavImages WHERE user_id=$1 and image=$2", args

