using System.IO;
using System.Threading.Tasks;

namespace WaifuApi.Application.Interfaces;

public record ImageMetadata(
    string PerceptualHash,
    string DominantColor,
    int Width,
    int Height,
    long ByteSize,
    string Extension,
    bool IsAnimated
);

public interface IImageProcessingService
{
    Task<ImageMetadata> ProcessAsync(Stream stream, string fileName);
}
