using System.IO;
using System.Threading;
using System.Threading.Tasks;

namespace WaifuApi.Application.Interfaces;

/// <summary>
/// The result of a successful format conversion. <see cref="Content"/> is a
/// rewound stream the caller owns, so dispose the record when finished with it.
/// </summary>
public record ImageConversionResult(
    Stream Content,
    string Extension,
    string ContentType,
    long ByteSize
) : IDisposable
{
    public void Dispose() => Content.Dispose();
}

/// <summary>
/// Re-encodes an upload into a cheaper-to-serve format before it reaches
/// storage. Distinct from <see cref="IImageProcessingService"/>, which only
/// reads metadata and must always see the original bytes so the perceptual
/// hash and duplicate detection stay stable.
/// </summary>
public interface IImageConverter
{
    /// <summary>
    /// Attempts to convert <paramref name="stream"/> to WebP. Returns
    /// <c>null</c> when conversion is disabled, the format is not eligible, the
    /// image is animated, or conversion failed - in every one of those cases the
    /// caller stores the original bytes unchanged.
    /// </summary>
    Task<ImageConversionResult?> TryConvertAsync(
        Stream stream,
        string extension,
        bool isAnimated,
        CancellationToken cancellationToken);
}
