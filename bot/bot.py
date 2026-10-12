import os
import logging
import asyncio
import random
import time
import urllib.parse
import discord
from discord.ext import commands
import aiohttp
from dotenv import load_dotenv

load_dotenv()

# Default to the Cloudflare Worker rather than the API host directly.
#
# The worker caches anonymous reads at the edge for 60s, so a repeat command is
# answered from the cache instead of the database. The API host is responsive in
# its own right; the caching is the advantage, not a workaround for a sleeping
# backend. The response shape is identical either way.
API_BASE = os.getenv("WAIFU_API", "https://waifu-cdn.waifu-api.workers.dev").rstrip("/")
PREFIX = os.getenv("PREFIX", "!")
NSFW_CHANNELS = {int(c) for c in os.getenv("NSFW_CHANNELS", "").split(",") if c.strip().lstrip("-").isdigit()}

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("waifu")

intents = discord.Intents.default()
# Prefix commands read message text, so the MESSAGE CONTENT privileged intent must be
# switched on in the Discord developer portal (Bot > Privileged Gateway Intents).
intents.message_content = True

session: aiohttp.ClientSession | None = None
ORIENTATIONS = {"landscape", "portrait", "square"}


class WaifuApiError(Exception):
    """The API answered, but with an error status. `status` is the HTTP code."""

    def __init__(self, message, status=None):
        super().__init__(message)
        self.status = status


def nsfw_allowed(channel):
    """A channel may serve NSFW results if Discord itself age-gates it, or if its
    ID is in the NSFW_CHANNELS allowlist (for channels the admin didn't mark)."""
    return channel.is_nsfw() or channel.id in NSFW_CHANNELS


def nsfw_hint(channel):
    return (
        f"This channel isn't marked age-restricted. Ask a server admin to enable "
        f"*Channel Settings > Age Restriction*, or add the channel ID "
        f"({channel.id}) to `NSFW_CHANNELS` in the bot config."
    )


def command_usage(prefix, cmd):
    args = cmd.usage or cmd.signature or ""
    return f"{prefix}{cmd.qualified_name} {args}".strip()


class WaifuHelp(commands.HelpCommand):
    """!help — list commands, or !help <command> for details."""

    async def send_bot_help(self, mapping):
        ctx = self.context
        embed = discord.Embed(
            title=f"{ctx.me.name} — commands",
            description="Powered by the [waifu.im](https://waifu-api-eosin.vercel.app) API.",
            color=discord.Color.magenta(),
        )
        for cmd in sorted(self.context.bot.commands, key=lambda c: c.name):
            if cmd.hidden or not cmd.short_doc:
                continue
            embed.add_field(
                name=f"`{command_usage(ctx.clean_prefix, cmd)}`", value=cmd.short_doc, inline=False
            )
        embed.set_footer(text=f"Run {ctx.clean_prefix}help <command> for details.")
        await ctx.send(embed=embed)

    async def send_command_help(self, command):
        ctx = self.context
        signature = command_usage(ctx.clean_prefix, command)
        embed = discord.Embed(
            title=f"`{signature}`",
            description=command.help or "No description.",
            color=discord.Color.magenta(),
        )
        if command.aliases:
            embed.add_field(name="Aliases", value=", ".join(f"`{a}`" for a in command.aliases))
        await ctx.send(embed=embed)


bot = commands.Bot(command_prefix=PREFIX, intents=intents, help_command=WaifuHelp())

# When the process started, for the uptime line.
STARTED_AT = time.monotonic()

# Numbers the status line and !stats report, gathered as the bot goes about its
# business rather than by asking the API when someone is watching.
#
# Written by the probe task, by every api_get, and by the commands that hand an
# image out; read by current_status. Every value is optional except the counters:
# a line that has nothing to show returns None and falls back to a fixed one.
STATS = {
    "latency_ms": None,      # last successful round trip to API_BASE
    "images": None,          # /stats/public totalImages
    "tags": None,            # /stats/public totalTags
    "artists": None,         # /stats/public totalArtists
    "served": 0,             # images this process has posted
    "last_id": None,         # ID of the most recent image posted
    "errors": 0,             # failed API calls since start
    "last_error_at": None,   # monotonic, or None if nothing has failed yet
}


def note_ok(latency_ms):
    STATS["latency_ms"] = round(latency_ms)


def note_error():
    STATS["errors"] += 1
    STATS["last_error_at"] = time.monotonic()


def note_served(img):
    STATS["served"] += 1
    STATS["last_id"] = img.get("id")


async def api_get(path, **params):
    """GET a path from the API host, returning the decoded body.

    Every API call goes through here so latency and errors are recorded wherever
    they happen, instead of only in the places that remember to.

    The body is JSON when the response says it is and text otherwise: /ping
    answers `pong` as text/plain, so decoding unconditionally would turn a
    healthy API into a raised ContentTypeError.
    """
    global session
    sess = session
    if sess is None or sess.closed:
        sess = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=30))
    # urlencode, not string joining: a tag containing a space, "&" or "#"
    # would otherwise produce a malformed URL or, worse, silently inject an
    # extra parameter. Today's slugs are all lowercase and hyphenated so nothing
    # trips it yet, but tags come from whatever users upload.
    query = urllib.parse.urlencode(
        {k: v for k, v in params.items() if v not in (None, "", [])}
    )
    url = f"{API_BASE}/{path.lstrip('/')}"
    if query:
        url = f"{url}?{query}"
    started = time.monotonic()
    try:
        async with sess.get(url) as resp:
            if resp.status >= 400:
                note_error()
                raise WaifuApiError(
                    f"The waifu API returned HTTP {resp.status}.", status=resp.status
                )
            if resp.content_type == "application/json":
                body = await resp.json(content_type=None)
            else:
                body = await resp.text()
            note_ok((time.monotonic() - started) * 1000)
            return body
    except WaifuApiError:
        raise
    except Exception:
        note_error()
        raise
    finally:
        if sess is not session and not sess.closed:
            await sess.close()
        elif sess.closed:
            session = None


async def fetch_images(**params):
    return await api_get("images", **params)


def build_embed(img):
    artist = next((a.get("name") for a in img.get("artists") or []), None)
    tags = [t.get("name") for t in img.get("tags") or []]
    embed = discord.Embed(color=discord.Color.magenta())
    embed.set_image(url=img["url"])
    embed.add_field(name="ID", value=str(img["id"]))
    if tags:
        embed.add_field(name="Tags", value=", ".join(tags))
    if artist:
        embed.add_field(name="Artist", value=artist)
    if img.get("isAnimated"):
        embed.add_field(name="Format", value="GIF")
    if img.get("source"):
        embed.add_field(name="Source", value=f"[link]({img['source']})")
    embed.set_footer(text="waifu.im")
    return embed


@bot.command(name="waifu")
@commands.cooldown(1, 10, commands.BucketType.user)
async def waifu(ctx, *, query: str = ""):
    """Get a waifu. Optionally pass `nsfw`, `gif`, an orientation, or tag names."""
    parts = query.lower().split()

    is_nsfw = "nsfw" in parts or nsfw_allowed(ctx.channel)
    if "nsfw" in parts and not nsfw_allowed(ctx.channel):
        await ctx.reply(f"This channel isn't an NSFW channel. {nsfw_hint(ctx.channel)}")
        return
    if "nsfw" in parts:
        parts.remove("nsfw")

    params = {
        "IsNsfw": "True" if is_nsfw else "False",
        "IsAnimated": "All",
        "OrderBy": "Random",
        "PageSize": 1,
    }

    if "gif" in parts:
        parts.remove("gif")
        params["IsAnimated"] = "True"

    orientation = next((p for p in parts if p in ORIENTATIONS), None)
    if orientation:
        parts.remove(orientation)
        params["Orientation"] = orientation.capitalize()

    if parts:
        params["IncludedTags"] = "-".join(parts)

    async with ctx.typing():
        try:
            data = await fetch_images(**params)
        except WaifuApiError as e:
            await ctx.reply(str(e))
            return
        except Exception as e:
            log.exception("image fetch failed")
            await ctx.reply(f"Could not reach the waifu API: {e}")
            return

    items = data.get("items") or []
    if not items:
        await ctx.reply("No images matched that search.")
        return
    note_served(items[0])
    await ctx.send(embed=build_embed(items[0]))


class GalleryView(discord.ui.View):
    def __init__(self, images, user):
        super().__init__(timeout=180)
        self.images = images
        self.index = 0
        self.user = user

    async def interaction_check(self, interaction: discord.Interaction) -> bool:
        if interaction.user.id != self.user.id:
            await interaction.response.send_message("Not your gallery.", ephemeral=True)
            return False
        return True

    @discord.ui.button(label="◀ Prev", style=discord.ButtonStyle.secondary)
    async def prev_button(self, interaction: discord.Interaction, button: discord.ui.Button):
        self.index = (self.index - 1) % len(self.images)
        await interaction.response.edit_message(embed=build_embed(self.images[self.index]), view=self)

    @discord.ui.button(label="▶ Next", style=discord.ButtonStyle.secondary)
    async def next_button(self, interaction: discord.Interaction, button: discord.ui.Button):
        self.index = (self.index + 1) % len(self.images)
        await interaction.response.edit_message(embed=build_embed(self.images[self.index]), view=self)

    async def on_timeout(self):
        for item in self.children:
            item.disabled = True
        msg = await self.message.channel.fetch_message(self.message.id)
        await msg.edit(view=self)


@bot.command(name="gallery")
@commands.cooldown(1, 15, commands.BucketType.user)
async def gallery(ctx):
    """Show a gallery of random waifus with prev/next buttons (valid 3 min)."""
    is_nsfw = nsfw_allowed(ctx.channel)
    params = {
        "IsNsfw": "True" if is_nsfw else "False",
        "IsAnimated": "All",
        "OrderBy": "Random",
        "PageSize": 5,
    }
    async with ctx.typing():
        try:
            data = await fetch_images(**params)
        except Exception as e:
            await ctx.reply(f"Could not reach the waifu API: {e}")
            return
    items = data.get("items") or []
    if not items:
        await ctx.reply("No images matched that search.")
        return
    view = GalleryView(items, ctx.author)
    msg = await ctx.send(embed=build_embed(items[0]), view=view)
    view.message = msg
    note_served(items[0])


@bot.command(name="tags")
@commands.cooldown(1, 10, commands.BucketType.user)
async def tags(ctx):
    """List the tags the API knows about, with image counts."""
    try:
        data = await api_get("tags")
    except Exception as e:
        await ctx.reply(f"Could not reach the waifu API: {e}")
        return

    body = "\n".join(
        f"`{t['slug']}` ({t['imageCount']})" for t in data.get("items") or [] if t.get("imageCount")
    )
    await ctx.send(
        embed=discord.Embed(title="Tags", description=body or "none", color=discord.Color.magenta())
    )


@bot.command(name="ping")
async def ping(ctx):
    """Check the bot and the waifu API are both alive."""
    try:
        await api_get("ping")
        status = "up"
    except WaifuApiError as e:
        status = f"HTTP {e.status}"
    except Exception as e:
        status = f"unreachable ({e})"
    await ctx.reply(f"waifu API: {status} | bot latency: {round(bot.latency * 1000)}ms")


@bot.command(name="stats")
async def stats_cmd(ctx):
    """Live numbers: API latency, catalogue size, uptime and error count."""
    embed = discord.Embed(title="Stats", color=discord.Color.magenta())
    embed.add_field(name="API latency", value=f"{STATS['latency_ms']}ms" if STATS["latency_ms"] else "no reading yet")
    embed.add_field(name="Uptime", value=f"{human_duration(time.monotonic() - STARTED_AT)}")
    embed.add_field(name="Served here", value=f"{STATS['served']} images")
    if STATS["errors"]:
        embed.add_field(name="API errors", value=f"{STATS['errors']} since start")
    for key, label in (("images", "Images"), ("tags", "Tags"), ("artists", "Artists")):
        if STATS[key]:
            embed.add_field(name=label, value=f"{STATS[key]:,}")
    embed.set_footer(text=f"Host: {API_BASE}")
    await ctx.reply(embed=embed)


@bot.event
async def on_disconnect():
    pass


@bot.event
async def on_ready():
    global session
    session = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=30))
    log.info("logged in as %s", bot.user)
    bot.loop.create_task(update_status())
    bot.loop.create_task(probe_api())


# What the bot was last asked to do, and when.
#
# Recorded from a single on_command_completion hook rather than inside each
# command, so a new command is picked up for free.
_last_activity = {"command": None, "at": 0.0}

# How long a command counts as "recent" for the reactive status slot.
ACTIVITY_MEMORY_SECONDS = 30 * 60

# The rotating status lines. Activity types are varied on purpose: four
# consecutive identical types read as a stuck bot.
#
# A None entry is a live slot: it reports a number from STATS and falls back to
# a fixed line until there is a number to report. Slots holding a tuple are
# always that line, whatever the API is doing.
STATUS_ROTATION = [
    None,  # API latency
    (discord.ActivityType.playing, "routing tags since 2021"),
    None,  # catalogue size
    None,  # reactive: the last command used
    (discord.ActivityType.playing, "hunting duplicates at Hamming <= 4"),
    None,  # uptime and errors
    (discord.ActivityType.competing, "PNG vs WebP, and the winner is WebP"),
    (discord.ActivityType.listening, "C should have gone in the tags"),
]

# Spare lines, used when a live slot has nothing to say yet.
#
# Kept disjoint from STATUS_ROTATION, or the same line can appear two slots
# apart and read as a stuck bot.
FALLBACK_STATUSES = [
    (discord.ActivityType.playing, "resizing things that were already the right size"),
    (discord.ActivityType.listening, "the sound of a cold start"),
    (discord.ActivityType.competing, "your browser's connection pool"),
    (discord.ActivityType.watching, "the Cloudflare edge, probably"),
]

# Status lines that react to the last command used. This is what makes the
# presence feel awake rather than ticking along on a timer.
ACTIVITY_STATUSES = {
    "waifu": (discord.ActivityType.watching, "one waifu at a time"),
    "gallery": (discord.ActivityType.watching, "five images, three minutes, no pressure"),
    "tags": (discord.ActivityType.competing, "the entire tag list, top to bottom"),
    "ping": (discord.ActivityType.listening, "waiting on the API, as usual"),
    "stats": (discord.ActivityType.watching, "its own homework"),
}

# How often to re-measure, for the lines that should not go stale while nobody
# is talking to the bot.
#
# The edge caches anonymous reads for 60s, so a 60s probe costs one origin
# request a minute at worst. /stats/public barely changes, so it is read far
# less often than it is asked for.
PROBE_SECONDS = 60
COUNT_REFRESH_SECONDS = 30 * 60

# A failure this recent is worth reporting even if later calls have succeeded;
# older than this it is history, and the uptime line takes over.
ERROR_WINDOW_SECONDS = 15 * 60


@bot.event
async def on_command_completion(ctx):
    _last_activity["command"] = ctx.command.name
    _last_activity["at"] = time.monotonic()


def human_duration(seconds):
    """A duration as one short unit: 45s, 12m, 6h, 3d. Discord limits a status
    to 128 characters, so there is no room for '3 days, 4 hours'."""
    seconds = int(seconds)
    if seconds < 60:
        return f"{seconds}s"
    if seconds < 3600:
        return f"{seconds // 60}m"
    if seconds < 86400:
        return f"{seconds // 3600}h"
    return f"{seconds // 86400}d"


def latency_status():
    if STATS["latency_ms"] is None:
        return None
    ms = STATS["latency_ms"]
    if ms < 100:
        return (discord.ActivityType.watching, f"the API, {ms}ms away")
    if ms < 500:
        return (discord.ActivityType.listening, f"the API, {ms}ms away and thinking")
    return (discord.ActivityType.playing, f"the API, {ms}ms away — Render's coffee is cold")


def catalogue_status():
    """Cycle through whichever totals /stats/public has given us.

    One number is a fact; four are a pitch. They arrive together and mean the
    same thing, so picking among them keeps a repeated line from reading as a
    stuck bot.
    """
    available = [
        (STATS["images"], "{:,} waifus and still counting"),
        (STATS["tags"], "{:,} tags, all of them load-bearing"),
        (STATS["artists"], "{:,} artists, quietly going uncredited"),
    ]
    available = [(value, template) for value, template in available if value]
    if not available:
        return None
    value, template = random.choice(available)
    return discord.ActivityType.watching, template.format(value)


def service_status():
    """Uptime and errors. The error line wins while it is fresh, because 'is it
    broken right now' is the question this line exists to answer."""
    since_error = (
        None if STATS["last_error_at"] is None
        else time.monotonic() - STATS["last_error_at"]
    )
    if since_error is not None and since_error < ERROR_WINDOW_SECONDS:
        ago = human_duration(since_error)
        if STATS["latency_ms"] is not None:
            return (discord.ActivityType.watching, f"last error {ago} ago, still serving")
        return (discord.ActivityType.playing, f"last error {ago} ago, still trying")

    uptime = human_duration(time.monotonic() - STARTED_AT)
    served = STATS["served"]
    if served:
        return (discord.ActivityType.playing, f"up {uptime}, {served} served")
    return (discord.ActivityType.listening, f"up {uptime}, waiting to be useful")


LIVE_STATUSES = {0: latency_status, 2: catalogue_status, 5: service_status}


def current_status(slot):
    """Return the (type, name) to show for this slot of the rotation.

    Slots 0, 2 and 5 read numbers from STATS; slot 3 reacts to the last command;
    the rest are fixed lines. A live slot with nothing to report falls back to a
    spare line, so the status never depends on the API being up or on someone
    having typed a command.
    """
    slot %= len(STATUS_ROTATION)

    live = LIVE_STATUSES.get(slot)
    if live is not None:
        return live() or random.choice(FALLBACK_STATUSES)

    entry = STATUS_ROTATION[slot]
    if entry is not None:
        return entry

    command = _last_activity["command"]
    recent = time.monotonic() - _last_activity["at"] < ACTIVITY_MEMORY_SECONDS
    if recent and command in ACTIVITY_STATUSES:
        return ACTIVITY_STATUSES[command]
    return random.choice(FALLBACK_STATUSES)


async def update_status():
    await bot.wait_until_ready()
    slot = 0
    while not bot.is_closed():
        try:
            activity_type, name = current_status(slot)
            await bot.change_presence(
                activity=discord.Activity(name=name, type=activity_type)
            )
        except Exception as e:
            log.warning("status update failed: %s", e)

        slot += 1

        # Jitter the interval so the presence does not read as a cron job, and
        # stay well clear of Discord's limit of roughly two presence changes per
        # five minutes: the floor here is 480s.
        await asyncio.sleep(random.randint(480, 780))


async def probe_api():
    """Keep the live status lines current without waiting for someone to run a
    command, which is the whole reason they are worth showing.

    Two rates: /ping is cheap and tells us latency and whether the API is
    answering at all, /stats/public is a bigger payload and barely moves, so it
    is read far less often.
    """
    await bot.wait_until_ready()
    last_counts = 0.0
    while not bot.is_closed():
        try:
            await api_get("ping")
            if time.monotonic() - last_counts >= COUNT_REFRESH_SECONDS:
                # A failed refresh is already counted by api_get; the next round
                # retries, so there is nothing extra to record here.
                counts = await api_get("stats/public")
                STATS["images"] = counts.get("totalImages")
                STATS["tags"] = counts.get("totalTags")
                STATS["artists"] = counts.get("totalArtists")
                last_counts = time.monotonic()
        except Exception as e:
            log.warning("probe failed: %s", e)
        await asyncio.sleep(PROBE_SECONDS)


bot.run(os.environ["DISCORD_TOKEN"], log_handler=None)
