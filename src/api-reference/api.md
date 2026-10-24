The SkyCrypt API provides access to player profiles and other data. It is intended for use by developers who want to integrate SkyCrypt data into their own applications or services.

## Authentication
Profile endpoints need a SkyCrypt API key in the `x-api-key` header. Request one from the developer page in your SkyCrypt dashboard.
Each key has its own rate limit; responses carry `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `X-RateLimit-Reset` headers.

## Caching
Since the Hypixel API policy doesn't allow proxying, the API only returns players and profiles that SkyCrypt has already cached. Players and profiles that have not been cached by us will return a `404` error with the code `NOT_CACHED`.

If you need to get the latest data for a player or profile or your application can't work with this limitation, you should use the official Hypixel API instead. You can find the documentation for that at [https://api.hypixel.net](https://api.hypixel.net).

## Versioning
We are not in a position to support versioning, migrations, backwards compatibility, deprecation, announcing changes, or any of the other things that come with versioning.

We update our API as we see fit and that it fits with our own needs. Breaking changes are possible at any time, and we will not provide any notice or support for them.

If you need a stable API, you should use the [official Hypixel API](https://api.hypixel.net) instead.

## Specification
The API is documented using the OpenAPI specification. You can find the full raw specification at [/api/openapi/doc.json](/api/openapi/doc.json).

## Usage
To use our API, you must read and agree to our [API Policy](https://sky.shiiyu.moe/api-policy). By using the API, you agree to comply with these terms.

This API is not affiliated with or endorsed by Hypixel Studios, Mojang Studios, or Microsoft. All trademarks are property of their respective owners.