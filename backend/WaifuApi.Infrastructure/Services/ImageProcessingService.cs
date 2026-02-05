using System;
using System.IO;
using System.Threading.Tasks;
using CoenM.ImageHash.HashAlgorithms;
using Microsoft.Extensions.Configuration;
using SixLabors.ImageSharp;
using SixLabors.ImageSharp.PixelFormats;
using SixLabors.ImageSharp.Processing;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Infrastructure.Services;

public class ImageProcessingService : IImageProcessingService
{
    private readonly IConfiguration _configuration;
    private readonly PerceptualHash _perceptualHash;

    public ImageProcessingService(IConfiguration configuration)
    {
        _configuration = configuration;
        _perceptualHash = new PerceptualHash(); 
    }

    public async Task<ImageMetadata> ProcessAsync(Stream stream, string fileName)
    {
        stream.Position = 0;
        using var image = await Image.LoadAsync<Rgba32>(stream);
        
        var width = image.Width;
        var height = image.Height;

        var extension = Path.GetExtension(fileName).ToLower();
        var isAnimated = image.Frames.Count > 1;

        // Resolution Check — animated images (GIFs etc.) use separate limits, falling back to static ones
        var minWidth = int.Parse(
            (isAnimated ? _configuration["Image:AnimatedMinWidth"] : null)
            ?? _configuration["Image:MinWidth"]
            ?? throw new InvalidOperationException("Image:MinWidth is required."));
        var minHeight = int.Parse(
            (isAnimated ? _configuration["Image:AnimatedMinHeight"] : null)
            ?? _configuration["Image:MinHeight"]
            ?? throw new InvalidOperationException("Image:MinHeight is required."));
        var maxWidth = int.Parse(
            (isAnimated ? _configuration["Image:AnimatedMaxWidth"] : null)
            ?? _configuration["Image:MaxWidth"]
            ?? throw new InvalidOperationException("Image:MaxWidth is required."));
        var maxHeight = int.Parse(
            (isAnimated ? _configuration["Image:AnimatedMaxHeight"] : null)
            ?? _configuration["Image:MaxHeight"]
            ?? throw new InvalidOperationException("Image:MaxHeight is required."));

        if (width < minWidth || height < minHeight)
        {
            throw new InvalidOperationException(
                $"Image resolution is too low ({width}x{height}). Minimum is {minWidth}x{minHeight}{(isAnimated ? " for animated images" : "")}.");
        }

        if (width > maxWidth || height > maxHeight)
        {
            throw new InvalidOperationException(
                $"Image resolution is too high ({width}x{height}). Maximum is {maxWidth}x{maxHeight}{(isAnimated ? " for animated images" : "")}.");
        }
        var byteSize = stream.Length;

        // Dominant Color
        var dominantColor = GetDominantColor(image);

        // Perceptual Hash using CoenM.ImageHash
        var hash = _perceptualHash.Hash(image);

        return new ImageMetadata(
            hash.ToString("X16"),
            dominantColor,
            width,
            height,
            byteSize,
            extension,
            isAnimated
        );
    }

    private string GetDominantColor(Image<Rgba32> image)
    {
        using var clone = image.Clone(ctx => ctx.Resize(1, 1));
        var pixel = clone[0, 0];
        return $"#{pixel.R:X2}{pixel.G:X2}{pixel.B:X2}";
    }
}
