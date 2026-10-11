using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Logging.Abstractions;
using SixLabors.ImageSharp;
using SixLabors.ImageSharp.Formats.Png;
using SixLabors.ImageSharp.PixelFormats;
using WaifuApi.Application.Interfaces;
using WaifuApi.Infrastructure.Services;
using WaifuApi.Tests.Infrastructure;
using Xunit;

namespace WaifuApi.Tests;

/// <summary>
/// Covers <see cref="ImageConverter"/> in isolation, plus the upload/update
/// handlers' reaction to it. The point of the feature is that a failure to
/// convert must never fail an upload, so several of these assert the "do
/// nothing" paths rather than the happy one.
/// </summary>
public class ImageConverterTests
{
    // ---- ImageConverter unit tests -----------------------------------------

    private static ImageConverter BuildConverter(params (string Key, string? Value)[] config)
    {
        var settings = new Dictionary<string, string?>
        {
            ["Image:WebpConversionEnabled"] = "true"
        };
        foreach (var (key, value) in config)
        {
            settings[key] = value;
        }

        var configuration = new ConfigurationBuilder().AddInMemoryCollection(settings).Build();
        return new ImageConverter(configuration, NullLogger<ImageConverter>.Instance);
    }

    private static MemoryStream PngBytes(int width = 64, int height = 64)
    {
        var stream = new MemoryStream();
        using var image = new Image<Rgba32>(width, height, new Rgba32(200, 30, 30));
        image.Save(stream, new PngEncoder());
        stream.Position = 0;
        return stream;
    }

    /// <summary>
    /// Noise rather than a flat colour: a solid fill compresses to the same
    /// handful of bytes at any quality, so it cannot tell q20 from q95.
    /// </summary>
    private static MemoryStream DetailedPngBytes(int width = 160, int height = 160)
    {
        var random = new Random(42);
        var stream = new MemoryStream();
        using var image = new Image<Rgba32>(width, height);
        for (var y = 0; y < height; y++)
        {
            for (var x = 0; x < width; x++)
            {
                image[x, y] = new Rgba32((byte)random.Next(256), (byte)random.Next(256), (byte)random.Next(256));
            }
        }

        image.Save(stream, new PngEncoder());
        stream.Position = 0;
        return stream;
    }

    [Fact]
    public async Task ReturnsNull_WhenConversionIsDisabled()
    {
        var converter = new ImageConverter(
            new ConfigurationBuilder().AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Image:WebpConversionEnabled"] = "false"
            }).Build(),
            NullLogger<ImageConverter>.Instance);

        using var png = PngBytes();
        var result = await converter.TryConvertAsync(png, ".png", isAnimated: false, CancellationToken.None);

        Assert.Null(result);
    }

    [Fact]
    public async Task Converts_PngToWebp()
    {
        var converter = BuildConverter();

        using var png = PngBytes();
        using var result = await converter.TryConvertAsync(png, ".png", isAnimated: false, CancellationToken.None);

        Assert.NotNull(result);
        Assert.Equal(".webp", result.Extension);
        Assert.Equal("image/webp", result.ContentType);

        // The reported size must describe the bytes actually produced, because
        // that value is what gets written to Image.ByteSize.
        Assert.Equal(result.Content.Length, result.ByteSize);

        // And the payload must be a real WebP the platform can decode back.
        result.Content.Position = 0;
        using var decoded = await Image.LoadAsync(result.Content);
        Assert.Equal(64, decoded.Width);
        Assert.Equal(64, decoded.Height);
    }

    [Fact]
    public async Task ReturnsNull_ForAnimatedImages()
    {
        var converter = BuildConverter();

        using var png = PngBytes();
        var result = await converter.TryConvertAsync(png, ".png", isAnimated: true, CancellationToken.None);

        Assert.Null(result);
    }

    [Fact]
    public async Task ReturnsNull_ForJpeg_ByDefault()
    {
        // JPEG is already lossy-compressed, so re-encoding it is excluded unless
        // someone opts in explicitly.
        var converter = BuildConverter();

        using var source = PngBytes();
        var result = await converter.TryConvertAsync(source, ".jpg", isAnimated: false, CancellationToken.None);

        Assert.Null(result);
    }

    [Fact]
    public async Task Converts_Jpeg_WhenSourceExtensionsAreConfigured()
    {
        var converter = BuildConverter(("Image:WebpSourceExtensions", ".jpg"));

        using var source = PngBytes();
        using var result = await converter.TryConvertAsync(source, ".jpg", isAnimated: false, CancellationToken.None);

        Assert.NotNull(result);
        Assert.Equal(".webp", result.Extension);
    }

    [Fact]
    public async Task ReturnsNull_WhenTheBytesAreNotAnImage()
    {
        // This is the important resilience case: garbage that slipped past
        // validation must not turn into a 500 on upload.
        var converter = BuildConverter();

        using var garbage = new MemoryStream("this is definitely not an image"u8.ToArray());
        var result = await converter.TryConvertAsync(garbage, ".png", isAnimated: false, CancellationToken.None);

        Assert.Null(result);
    }

    [Fact]
    public async Task MatchesConfiguredQuality()
    {
        var high = BuildConverter(("Image:WebpQuality", "95"));
        var low = BuildConverter(("Image:WebpQuality", "20"));

        using var a = DetailedPngBytes();
        using var highResult = await high.TryConvertAsync(a, ".png", isAnimated: false, CancellationToken.None);

        using var b = DetailedPngBytes();
        using var lowResult = await low.TryConvertAsync(b, ".png", isAnimated: false, CancellationToken.None);

        Assert.NotNull(highResult);
        Assert.NotNull(lowResult);
        Assert.True(
            highResult.ByteSize > lowResult.ByteSize,
            $"q95 ({highResult.ByteSize}B) should be larger than q20 ({lowResult.ByteSize}B)");
    }

    // ---- handler integration ----------------------------------------------

    private static ImageMetadata MetadataFor(string extension, bool isAnimated = false, long byteSize = 4096) =>
        new("0000000000000000", "#000000", 64, 64, byteSize, extension, isAnimated);

    private static ImageConversionResult FakeConversion(long byteSize = 777)
    {
        var bytes = new byte[byteSize];
        bytes.AsSpan().Fill(0xAB);
        var stream = new MemoryStream(bytes);
        return new ImageConversionResult(stream, ".webp", "image/webp", byteSize);
    }

    [Fact]
    public async Task Upload_StoresOriginalBytes_WhenConverterDeclines()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync();

        harness.ImageProcessing.NextResult = MetadataFor(".png");
        harness.Converter.NextResult = null;

        using var source = PngBytes();
        var dto = await harness.SendAsync(new Application.Features.Images.UploadImage.UploadImageCommand(
            userId, source, "photo.png", "image/png", [], [], null, false, null));

        Assert.Equal(".png", dto.Extension);

        var upload = Assert.Single(harness.Storage.Uploads);
        Assert.EndsWith(".png", upload.FileName, StringComparison.Ordinal);
        Assert.Equal("image/png", upload.ContentType);
    }

    [Fact]
    public async Task Upload_StoresConvertedBytes_WhenConverterConverts()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync();

        var conversion = FakeConversion(byteSize: 777);
        harness.ImageProcessing.NextResult = MetadataFor(".png", byteSize: 4096);
        harness.Converter.NextResult = conversion;

        using var source = PngBytes();
        var dto = await harness.SendAsync(new Application.Features.Images.UploadImage.UploadImageCommand(
            userId, source, "photo.png", "image/png", [], [], null, false, null));

        // The row, the S3 key and the content type must all agree on WebP.
        Assert.Equal(".webp", dto.Extension);
        Assert.Equal(777, dto.ByteSize);

        var upload = Assert.Single(harness.Storage.Uploads);
        Assert.EndsWith(".webp", upload.FileName, StringComparison.Ordinal);
        Assert.Equal("image/webp", upload.ContentType);
        Assert.Equal(777, upload.Bytes.Length);

        var persisted = await harness.ReadAsync(db => db.Images
            .Where(i => i.Id == dto.Id)
            .Select(i => new { i.Extension, i.ByteSize })
            .FirstAsync());
        Assert.Equal(".webp", persisted.Extension);
        Assert.Equal(777, persisted.ByteSize);
    }

    [Fact]
    public async Task Update_StoresConvertedBytes_WhenFileIsReplaced()
    {
        using var harness = new TestHarness();
        var moderatorId = await harness.SeedUserAsync(Domain.Enums.Role.Moderator);
        var imageId = await harness.SeedImageAsync();

        var conversion = FakeConversion(byteSize: 512);
        harness.ImageProcessing.NextResult = MetadataFor(".png", byteSize: 4096);
        harness.Converter.NextResult = conversion;

        using var source = PngBytes();
        var dto = await harness.SendAsync(new Application.Features.Images.UpdateImage.UpdateImageCommand(
            imageId, null, false, null, null, null, null,
            source, "photo.png", "image/png"));

        Assert.Equal(".webp", dto.Extension);

        var upload = Assert.Single(harness.Storage.Uploads);
        Assert.EndsWith(".webp", upload.FileName, StringComparison.Ordinal);
        Assert.Equal("image/webp", upload.ContentType);

        // The converter is told about the source extension and animation flag,
        // never handed the already-converted bytes.
        var call = Assert.Single(harness.Converter.Calls);
        Assert.Equal(".png", call.Extension);
        Assert.False(call.IsAnimated);
    }
}
