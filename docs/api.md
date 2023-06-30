WAIFU.IM is an easy to use API that allows you to get waifu pictures from an archive of over 4000 images and multiple tags!

## Versioning

You can specify which version of the API you want to use by placing it in the `Accept-Version` header.

If the value doesn't match any version, the latest version is used.

We highly recommend using versioning to prevent your app from breaking if a new version with breaking changes is developed. You can check which version you are using in the `Version` response header.

The latest version of the API is `v5`.

## Breaking Changes

- `FAVOURITES` became `FAVORITES`.

- `/fav/delete` and `/fav/insert` now return the same structure as `/fav/toggle` and return a `200` status code.

- For more information, please join the Discord server or contact us by email.

## Rate Limit

Note: This rate limit only applies to the `api.waifu.im` domain.

The rate limit is **1** request every **200** milliseconds. If you exceed this limit, the request will be put in a queue (with a maximum size of **10**).

If the queue is full, the server will respond with a **429** status code.

You can check the `Retry-After` header to get the time to wait (in seconds) before making another request.

## Resources

Here are some useful resources related to the API:

- [GitHub](https://github.com/Waifu-im)

- [Python wrapper](https://github.com/Waifu-im/waifuim.py)

- [C# wrapper](https://github.com/SoNearSonar/WaifuImAPI-NET)

## Tags

Images are classified by a tag system and can have multiple tags!

There are versatile tags that can describe both safe and lewd images, as well as NSFW tags that only describe lewd images.

You can get a list of all tags at the `/tags` endpoint.

## Generate an Authorization Link

This part isn't directly related to the API, and the base URL is not the same.

This will allow you to generate a link asking a user to click on it and grant permissions over their Waifu.im account (you can also do the opposite with revoke).

This will be useful if you want to use the `user_id` query string when dealing with favorites.

| URI                                        | Request type |
|--------------------------------------------|--------------|
| https://www.waifu.im/authorization/        | GET          |
| https://www.waifu.im/authorization/revoke/ | GET          |

### Query strings

| Name        | Required | Type         | Behavior                                                                                                  |
|-------------|----------|--------------|-----------------------------------------------------------------------------------------------------------|
| user_id     | Yes      | Integer      | The Discord user ID of the user that will receive the permissions                                         |
| permissions | Yes      | String Array | The permissions that will be asked for. Available permissions are `view_favorites` and `manage_favorites` |

To take in an array of values, provide the query string again with different values.


## ⚠️ Warning
If you want to try routes that require authentication, you need to include your token. You can find your token at [https://www.waifu.im/dashboard](https://www.waifu.im/dashboard).

To include your token:

1. Click the `Authorize` button below.

2. In the dialog that appears, enter your token in the `Value` field.

3. Click `Authorize` to apply the token.

**Note:** When filling the field, ensure that you include the word `Bearer ` (with the space) before your token to properly authenticate your requests.