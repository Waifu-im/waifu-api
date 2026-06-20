using WaifuApi.Application.Interfaces;

namespace WaifuApi.Tests.Infrastructure;

/// <summary>No-op storage; the edit-request apply path never replaces files, so it is never exercised.</summary>
public class FakeStorageService : IStorageService
{
    public Task UploadAsync(Stream stream, string fileName, string contentType) => Task.CompletedTask;
    public Task DeleteAsync(string fileName) => Task.CompletedTask;
    public Task<List<string>> ListObjectsAsync(CancellationToken ct) => Task.FromResult(new List<string>());
}

/// <summary>Throws if called — edit-request approval never processes an image file.</summary>
public class FakeImageProcessingService : IImageProcessingService
{
    public Task<ImageMetadata> ProcessAsync(Stream stream, string fileName)
        => throw new InvalidOperationException("Image processing must not be invoked by the edit-request flow.");
}
