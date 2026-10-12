#!/usr/bin/env node
/**
 * Generates dist/sitemap.xml from the public image set plus the static indexable routes.
 *
 * Runs as a postbuild step, so it must never break the deploy: if the API is unreachable
 * it degrades to a static-only sitemap and warns loudly. Set SITEMAP_STRICT=1 to turn
 * that degradation into a hard failure (useful locally, and in CI).
 *
 * Env:
 *   SITE_URL        canonical origin for loc URLs   (default: production Vercel host)
 *   SITEMAP_API_URL base URL for the images feed    (default: the edge-cached CDN)
 *   SITEMAP_STRICT  "1" to fail instead of degrading
 */

import fs from 'node:fs/promises'
import path from 'node:path'

const SITE_URL = stripTrailingSlash(
  process.env.SITE_URL || process.env.VITE_SITE_URL || 'https://waifu-api-eosin.vercel.app',
)
const API_URL = stripTrailingSlash(
  process.env.SITEMAP_API_URL || process.env.VITE_API_URL || 'https://waifu-cdn.waifu-api.workers.dev',
)
const OUT_FILE = path.resolve(process.cwd(), 'dist', 'sitemap.xml')
const STRICT = process.env.SITEMAP_STRICT === '1'

const PAGE_SIZE = 500
const MAX_ATTEMPTS = 3
const TIMEOUT_MS = 30_000

// Static routes worth indexing. Gallery filter permutations (/gallery?tags=...) are
// deliberately absent -- they are the same content under different URLs and want a
// canonical tag, not a sitemap entry.
const STATIC_ROUTES = [
  { loc: '/', priority: '1.0', changefreq: 'daily' },
  { loc: '/gallery', priority: '0.9', changefreq: 'daily' },
  { loc: '/tags', priority: '0.6', changefreq: 'weekly' },
  { loc: '/artists', priority: '0.6', changefreq: 'weekly' },
  { loc: '/contact', priority: '0.3', changefreq: 'yearly' },
]

function stripTrailingSlash(value) {
  return value.replace(/\/+$/, '')
}

function escapeXml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;')
}

async function fetchJson(url) {
  let lastError

  for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), TIMEOUT_MS)
    try {
      const response = await fetch(url, {
        signal: controller.signal,
        headers: { Accept: 'application/json' },
      })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status} ${response.statusText}`)
      }
      return await response.json()
    } catch (error) {
      lastError = error
      if (attempt < MAX_ATTEMPTS) {
        const backoff = 1000 * 2 ** (attempt - 1)
        console.warn(`[sitemap] attempt ${attempt}/${MAX_ATTEMPTS} failed (${error.message}); retrying in ${backoff}ms`)
        await sleep(backoff)
      }
    } finally {
      clearTimeout(timer)
    }
  }

  throw lastError
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function fetchAllImages() {
  const images = []
  let page = 1
  let totalPages = 1

  // UploadedAt rather than the default Random so the output is deterministic: a diff
  // between builds then means content actually changed, rather than a reshuffle.
  do {
    const url = `${API_URL}/images?page=${page}&pageSize=${PAGE_SIZE}&orderBy=UploadedAt`
    const payload = await fetchJson(url)
    if (!payload || !Array.isArray(payload.items)) {
      throw new Error(`unexpected /images payload on page ${page}`)
    }
    images.push(...payload.items)
    totalPages = payload.totalPages ?? 1
    console.log(`[sitemap] fetched page ${page}/${totalPages} (${images.length} images)`)
    page += 1
  } while (page <= totalPages)

  return images
}

function imageTitle(image) {
  const artists = (image.artists ?? []).map((a) => a.name).filter(Boolean)
  if (artists.length > 0) return artists.join(', ')

  const tags = (image.tags ?? []).slice(0, 3).map((t) => t.name).filter(Boolean)
  if (tags.length > 0) return tags.join(', ')

  return `Waifu image #${image.id}`
}

// uploadedAt is an ISO timestamp; sitemaps only want the date part.
function toLastmod(uploadedAt) {
  const match = /^(\d{4}-\d{2}-\d{2})/.exec(uploadedAt ?? '')
  return match ? match[1] : null
}

function renderUrl({ loc, lastmod, changefreq, priority, image }) {
  const parts = [`    <loc>${escapeXml(loc)}</loc>`]

  if (lastmod) parts.push(`    <lastmod>${escapeXml(lastmod)}</lastmod>`)
  if (changefreq) parts.push(`    <changefreq>${escapeXml(changefreq)}</changefreq>`)
  if (priority) parts.push(`    <priority>${escapeXml(priority)}</priority>`)

  if (image) {
    parts.push('    <image:image>')
    parts.push(`      <image:loc>${escapeXml(image.loc)}</image:loc>`)
    parts.push(`      <image:title>${escapeXml(image.title)}</image:title>`)
    parts.push('    </image:image>')
  }

  return `  <url>\n${parts.join('\n')}\n  </url>`
}

function renderSitemap(entries) {
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">',
    ...entries.map(renderUrl),
    '</urlset>',
    '',
  ].join('\n')
}

async function main() {
  let images = []
  let degraded = false

  try {
    images = await fetchAllImages()
  } catch (error) {
    if (STRICT) throw error
    degraded = true
    console.warn(
      `[sitemap] WARNING: could not reach ${API_URL} (${error.message}). ` +
        'Writing a static-only sitemap; no image URLs will be submitted. ' +
        'Re-run with SITEMAP_STRICT=1 to make this fatal.',
    )
  }

  const entries = [
    ...STATIC_ROUTES.map((route) => ({
      loc: `${SITE_URL}${route.loc}`,
      changefreq: route.changefreq,
      priority: route.priority,
    })),
    ...images.map((image) => ({
      loc: `${SITE_URL}/images/${image.id}`,
      lastmod: toLastmod(image.uploadedAt),
      changefreq: 'monthly',
      priority: '0.7',
      image: { loc: image.url, title: imageTitle(image) },
    })),
  ]

  await fs.mkdir(path.dirname(OUT_FILE), { recursive: true })
  await fs.writeFile(OUT_FILE, renderSitemap(entries), 'utf8')

  console.log(
    `[sitemap] wrote ${entries.length} URLs to ${path.relative(process.cwd(), OUT_FILE)} ` +
      `(${images.length} image pages${degraded ? ', DEGRADED' : ''})`,
  )
}

main().catch((error) => {
  console.error(`[sitemap] failed: ${error.stack || error.message}`)
  process.exit(1)
})
