// Build a link to the upstream metadata source for an author or book, à la
// the *arr stacks' TMDB/IMDB links (#1296). The provider is implied by the
// foreign-ID prefix, matching the backend convention in
// internal/metadata/aggregator_providers.go and models.AuthorProviderFromForeignID.
//
// We only emit a link when the public URL can be constructed reliably from the
// stored ID. OpenLibrary (bare OL keys), Google Books (gb:) and Hardcover (hc:)
// qualify; DNB (dnb:), Calibre and Audiobookshelf do not — their stored IDs
// don't map to a stable public page — so those return null rather than risk a
// dead link.

export type MetadataSourceLink = { url: string; label: string }

export function metadataSourceLink(
  foreignId: string | undefined | null,
  kind: 'author' | 'book',
): MetadataSourceLink | null {
  const id = (foreignId ?? '').trim()
  if (!id) return null

  if (id.startsWith('gb:')) {
    const vol = id.slice(3).trim()
    // Google Books has no canonical author page; only books map cleanly.
    if (kind !== 'book' || !vol) return null
    return { url: `https://books.google.com/books?id=${encodeURIComponent(vol)}`, label: 'Google Books' }
  }

  // Hardcover stores the slug its own site routes on, so both kinds map to a
  // page: hardcover.app/books/<slug> and /authors/<slug>.
  //
  // Except when the record had no slug. `toBook` and `toAuthor` fall back to
  // the numeric primary key there (internal/metadata/hardcover/client.go), and
  // hardcover.app 404s on those, so an all-digit value gets no link. A numeric
  // slug is possible and would be losing a working link, but the two are
  // indistinguishable from the stored ID alone and a dead link is the worse of
  // the two failures.
  if (id.startsWith('hc:')) {
    const slug = id.slice(3).trim()
    if (!slug || /^\d+$/.test(slug)) return null
    const path = kind === 'author' ? 'authors' : 'books'
    return { url: `https://hardcover.app/${path}/${encodeURIComponent(slug)}`, label: 'Hardcover' }
  }

  // No reliable public URL for these providers.
  if (id.startsWith('dnb:') || id.startsWith('abs:') || id.startsWith('calibre:')) {
    return null
  }

  // Default: OpenLibrary, whose foreign IDs are bare OL keys. Author keys end
  // in A, work keys in W, edition keys in M.
  if (kind === 'author') {
    if (!/^OL\w+A$/i.test(id)) return null
    return { url: `https://openlibrary.org/authors/${id}`, label: 'OpenLibrary' }
  }
  if (/^OL\w+W$/i.test(id)) return { url: `https://openlibrary.org/works/${id}`, label: 'OpenLibrary' }
  if (/^OL\w+M$/i.test(id)) return { url: `https://openlibrary.org/books/${id}`, label: 'OpenLibrary' }
  return null
}

// Human-readable name for a metadata provider key as stored in
// books.metadata_provider / book_identifiers.provider (#1707). Unknown keys are
// returned as-is so a provider added on the backend still shows something
// truthful instead of being hidden.
const PROVIDER_NAMES: Record<string, string> = {
  openlibrary: 'OpenLibrary',
  hardcover: 'Hardcover',
  googlebooks: 'Google Books',
  dnb: 'DNB',
  calibre: 'Calibre',
  audiobookshelf: 'Audiobookshelf',
}

export function providerDisplayName(provider: string | undefined | null): string {
  const key = (provider ?? '').trim().toLowerCase()
  if (!key) return ''
  return PROVIDER_NAMES[key] ?? provider!.trim()
}

// Which provider a book foreign ID belongs to, mirroring
// models.BookProviderFromForeignID so the page can name the source even for
// rows whose metadata_provider column was never written. An unprefixed ID is
// OpenLibrary, matching the long-standing books.foreign_id convention.
export function providerFromBookForeignId(foreignId: string | undefined | null): string {
  const id = (foreignId ?? '').trim().toLowerCase()
  if (id.startsWith('gb:')) return 'googlebooks'
  if (id.startsWith('hc:')) return 'hardcover'
  if (id.startsWith('dnb:')) return 'dnb'
  if (id.startsWith('calibre:')) return 'calibre'
  if (id.startsWith('abs:')) return 'audiobookshelf'
  return 'openlibrary'
}

// Public page for a Hardcover series (#1708).
//
// hardcover.app routes series on their slug, not on the numeric id Bindery
// stores as hardcoverProviderId, so there is nothing to build a link from until
// the slug is known. Returns null in that case and the caller renders plain
// text, which is the same "no reliable public URL, no link" rule
// metadataSourceLink follows.
export function hardcoverSeriesUrl(slug: string | undefined | null): string | null {
  const value = (slug ?? '').trim()
  if (!value) return null
  return `https://hardcover.app/series/${encodeURIComponent(value)}`
}

// Display name for a metadata provider.
//
// The backend normalizes provider names in
// internal/metadata/aggregator_providers.go and stamps one onto every search
// result, so a result list that mixes providers can say which is which. The
// stored value is a lower-case key; this is the name a reader recognises.
//
// Returns null when the provider is unknown, so a caller can leave the label
// out rather than print a key nobody set.
export function providerLabel(
  provider: string | undefined | null,
  foreignId?: string | undefined | null,
): string | null {
  switch ((provider ?? '').trim().toLowerCase()) {
    case 'hardcover':
      return 'Hardcover'
    case 'openlibrary':
      return 'OpenLibrary'
    case 'googlebooks':
      return 'Google Books'
    case 'dnb':
      return 'DNB'
    case 'calibre':
      return 'Calibre'
    case 'audiobookshelf':
      return 'Audiobookshelf'
  }

  // Older records predate the provider column; their foreign ID still carries
  // the prefix the backend assigns.
  const id = (foreignId ?? '').trim()
  if (id.startsWith('hc:')) return 'Hardcover'
  if (id.startsWith('gb:')) return 'Google Books'
  if (id.startsWith('dnb:')) return 'DNB'
  if (id.startsWith('abs:')) return 'Audiobookshelf'
  if (id.startsWith('calibre:')) return 'Calibre'
  if (/^OL\w+[AWM]$/i.test(id)) return 'OpenLibrary'

  const raw = (provider ?? '').trim()
  return raw === '' ? null : raw
}
