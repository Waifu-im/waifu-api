using System;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Logging;
using SixLabors.ImageSharp;
using SixLabors.ImageSharp.Formats.Webp;
using SixLabors.ImageSharp.PixelFormats;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Infrastructure.Services;

/// <summary>
/// Converts uploads to WebP at ingest so that new images benefit from the
/// smaller file size without anyone having to run a batch job afterwards.
/// Conversion is opt-in via <c>Image:WebpConversionEnabled</c> and is limited to
/// PNG sources by default: JPEG is already lossy-compressed, so re-encoding it
/// is lossy-to-lossy and risks visible generation loss.
/// </summary>
public class ImageConverter : IImageConverter
{
    public const string WebpExtension = ".webp";
    public const string WebpContentType = "image/webp";

    private const int DefaultQuality = 90;

    /// <summary>
    /// Comma-separated rather than a config array: binding a single-element
    /// string[] from IConfiguration is fiddly, and ".png" reads better in
    /// appsettings.json than an indexed list.
    /// </summary>
    private const string DefaultSourceExtensions = ".png";

    private readonly IConfiguration _configuration;
    private readonly ILogger<ImageConverter> _logger;

    public ImageConverter(IConfiguration configuration, ILogger<ImageConverter> logger)
    {
        _configuration = configuration;
        _logger = logger;
    }

    public async Task<ImageConversionResult?> TryConvertAsync(
        Stream stream,
        string extension,
        bool isAnimated,
        CancellationToken cancellationToken)
    {
        if (!_configuration.GetValue<bool>("Image:WebpConversionEnabled"))
        {
            return null;
        }

        // Animated images are left alone. Animated WebP encoding is not a
        // dependency we want here yet, and re-compressing a GIF's palette into
        // a lossy format is the exact quality regression this feature exists to
        // avoid.
        if (isAnimated)
        {
            return null;
        }

        var eligibleExtensions = (_configuration["Image:WebpSourceExtensions"] ?? DefaultSourceExtensions)
            .Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);

        if (!eligibleExtensions.Contains(extension, StringComparer.OrdinalIgnoreCase))
        {
            return null;
        }

        try
        {
            stream.Position = 0;
            using var image = await Image.LoadAsync<Rgba32>(stream, cancellationToken);

            var quality = Math.Clamp(
                _configuration.GetValue<int?>("Image:WebpQuality") ?? DefaultQuality,
                1,
                100);

            var converted = new MemoryStream();
            await image.SaveAsync(
                converted,
                new WebpEncoder
                {
                    Quality = quality,
                    FileFormat = WebpFileFormatType.Lossy
                },
                cancellationToken);

            converted.Position = 0;

            return new ImageConversionResult(converted, WebpExtension, WebpContentType, converted.Length);
        }
        catch (Exception ex)
        {
            // Never fail an upload because re-encoding went wrong - the bytes the
            // caller sent are still a perfectly serviceable image.
            _logger.LogWarning(
                ex,
                "WebP conversion failed for {Extension}; storing the original bytes",
                extension);

            return null;
        }
    }
}
