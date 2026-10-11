using WaifuApi.Application.Interfaces;

namespace WaifuApi.Tests.Infrastructure;

/// <summary>No-op storage; the edit-request apply path never replaces files, so it is never exercised.</summary>
public class FakeStorageService : IStorageService
{
    /// <summary>Every UploadAsync call, in order, for assertions on what was actually stored.</summary>
    public List<(string FileName, string ContentType, byte[] Bytes)> Uploads { get; } = [];

    public Task UploadAsync(Stream stream, string fileName, string contentType)
    {
        using var buffer = new MemoryStream();
        stream.CopyTo(buffer);
        Uploads.Add((fileName, contentType, buffer.ToArray()));
        return Task.CompletedTask;
    }

    public Task DeleteAsync(string fileName) => Task.CompletedTask;
    public Task<List<string>> ListObjectsAsync(CancellationToken ct) => Task.FromResult(new List<string>());
}

/// <summary>
/// Reads metadata only. Throws unless a test sets <see cref="NextResult"/>, so
/// the edit-request flow still fails loudly if it ever starts processing files.
/// </summary>
public class FakeImageProcessingService : IImageProcessingService
{
    public ImageMetadata? NextResult { get; set; }

    public Task<ImageMetadata> ProcessAsync(Stream stream, string fileName)
        => NextResult is { } metadata
            ? Task.FromResult(metadata)
            : throw new InvalidOperationException("Image processing must not be invoked by the edit-request flow.");
}

/// <summary>
/// Converts nothing by default, matching the production default of conversion
/// being off. Tests that exercise conversion set <see cref="NextResult"/>.
/// </summary>
public class FakeImageConverter : IImageConverter
{
    /// <summary>What the next call should return. Null means "not converted".</summary>
    public ImageConversionResult? NextResult { get; set; }

    /// <summary>Extensions the converter was called with, for assertions.</summary>
    public List<(string Extension, bool IsAnimated)> Calls { get; } = [];

    public Task<ImageConversionResult?> TryConvertAsync(
        Stream stream, string extension, bool isAnimated, CancellationToken cancellationToken)
    {
        Calls.Add((extension, isAnimated));
        return Task.FromResult(NextResult);
    }
}

/// <summary>No-op CDN cache; nothing to purge in tests.</summary>
public class FakeCdnCacheService : ICdnCacheService
{
    public Task PurgeImageAsync(long imageId, IEnumerable<string> extensions, CancellationToken ct = default) => Task.CompletedTask;
    public Task PurgeFilesAsync(IEnumerable<string> fileNames, CancellationToken ct = default) => Task.CompletedTask;
}
