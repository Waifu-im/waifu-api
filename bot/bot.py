import os
import logging
import asyncio
import random
import time
import discord
from discord.ext import commands
import aiohttp
from dotenv import load_dotenv

load_dotenv()

# Default to the Cloudflare Worker rather than the API host directly.
#
# The worker caches anonymous reads at the edge for 60s, so this avoids both the
# round trip to Render and its free-tier cold starts, where a sleeping instance
# can return 502 mid-command. The response shape is identical either way.
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
    pass


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


async def fetch_images(**params):
    global session
    sess = session
    if sess is None or sess.closed:
        sess = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=30))
    query = "&".join(f"{k}={v}" for k, v in params.items() if v not in (None, "", []))
    try:
        async with sess.get(f"{API_BASE}/images?{query}") as resp:
            if resp.status >= 400:
                raise WaifuApiError(f"The waifu API returned HTTP {resp.status}.")
            return await resp.json()
    finally:
        if sess is not session and not sess.closed:
            await sess.close()
        elif sess.closed:
            session = None


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


@bot.command(name="tags")
@commands.cooldown(1, 10, commands.BucketType.user)
async def tags(ctx):
    """List the tags the API knows about, with image counts."""
    global session
    sess = session
    if sess is None or sess.closed:
        sess = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=30))
    try:
        async with sess.get(f"{API_BASE}/tags") as resp:
            data = await resp.json()
    except Exception as e:
        await ctx.reply(f"Could not reach the waifu API: {e}")
        return
    finally:
        if sess is not session and not sess.closed:
            await sess.close()
        elif sess.closed:
            session = None

    body = "\n".join(
        f"`{t['slug']}` ({t['imageCount']})" for t in data.get("items") or [] if t.get("imageCount")
    )
    await ctx.send(
        embed=discord.Embed(title="Tags", description=body or "none", color=discord.Color.magenta())
    )


@bot.command(name="ping")
async def ping(ctx):
    """Check the bot and the waifu API are both alive."""
    global session
    sess = session
    if sess is None or sess.closed:
        sess = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=30))
    try:
        async with sess.get(f"{API_BASE}/ping") as resp:
            status = "up" if resp.status < 400 else f"HTTP {resp.status}"
    except Exception as e:
        status = f"unreachable ({e})"
    finally:
        if sess is not session and not sess.closed:
            await sess.close()
        elif sess.closed:
            session = None
    await ctx.reply(f"waifu API: {status} | bot latency: {round(bot.latency * 1000)}ms")


@bot.event
async def on_disconnect():
    pass


@bot.event
async def on_ready():
    global session
    session = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=30))
    log.info("logged in as %s", bot.user)
    bot.loop.create_task(update_status())


# What the bot was last asked to do, and when.
#
# Recorded from a single on_command_completion hook rather than inside each
# command, so a new command is picked up for free.
_last_activity = {"command": None, "at": 0.0}

# How long a command counts as "recent" for the reactive status slot.
ACTIVITY_MEMORY_SECONDS = 30 * 60

# The rotating status lines. Activity types are varied on purpose: four
# consecutive identical types read as a stuck bot.
STATUS_ROTATION = [
    (discord.ActivityType.playing, "routing tags since 2021"),
    (discord.ActivityType.watching, "4,285 waifus and still counting"),
    (discord.ActivityType.listening, "C should have gone in the tags"),
    (discord.ActivityType.competing, "your browser's connection pool"),
    (discord.ActivityType.playing, "hunting duplicates at Hamming <= 4"),
]

# Spare lines, used only when the reactive slot has nothing recent to say.
FALLBACK_STATUSES = [
    (discord.ActivityType.playing, "resizing things that were already the right size"),
    (discord.ActivityType.listening, "the sound of a cold start"),
    (discord.ActivityType.competing, "PNG vs WebP, and the winner is WebP"),
    (discord.ActivityType.watching, "the Cloudflare edge, probably"),
]

# Status lines that react to the last command used. This is what makes the
# presence feel awake rather than ticking along on a timer.
ACTIVITY_STATUSES = {
    "waifu": (discord.ActivityType.watching, "one waifu at a time"),
    "gallery": (discord.ActivityType.watching, "five images, three minutes, no pressure"),
    "tags": (discord.ActivityType.competing, "the entire tag list, top to bottom"),
    "ping": (discord.ActivityType.listening, "waiting on the API, as usual"),
}


@bot.event
async def on_command_completion(ctx, command):
    _last_activity["command"] = command.name
    _last_activity["at"] = time.monotonic()


def current_status(slot):
    """Return the (type, name) to show for this slot of the rotation.

    Slot 3 is the reactive one: if a command has been used recently it reports
    what the bot is actually doing, otherwise it falls back to the fixed
    rotation so the status never depends on someone having typed a command.
    """
    activity_type, name = STATUS_ROTATION[slot % len(STATUS_ROTATION)]

    if slot == 3:
        command = _last_activity["command"]
        recent = time.monotonic() - _last_activity["at"] < ACTIVITY_MEMORY_SECONDS
        if recent and command in ACTIVITY_STATUSES:
            return ACTIVITY_STATUSES[command]
        name = random.choice(FALLBACK_STATUSES)[1]

    return activity_type, name


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


bot.run(os.environ["DISCORD_TOKEN"], log_handler=None)
