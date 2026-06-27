using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace WaifuApi.Application.Interfaces;

public interface ICdnCacheService
{
    // Purges the CDN cache for a single image's files (original + resized variants) and nothing else.
    // No-op when CDN purge credentials aren't configured.
    Task PurgeImageAsync(long imageId, IEnumerable<string> extensions, CancellationToken ct = default);

    // Purges by raw S3 file name ("{id}{ext}"), for the orphan-cleanup path that deletes files directly.
    Task PurgeFilesAsync(IEnumerable<string> fileNames, CancellationToken ct = default);
}
