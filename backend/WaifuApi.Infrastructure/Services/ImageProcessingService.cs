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

        // Resolution Check
        var minWidth = int.Parse(_configuration["Image:MinWidth"] ?? throw new InvalidOperationException("Image:MinWidth is required."));
        var minHeight = int.Parse(_configuration["Image:MinHeight"] ?? throw new InvalidOperationException("Image:MinHeight is required."));
        var maxWidth = int.Parse(_configuration["Image:MaxWidth"] ?? throw new InvalidOperationException("Image:MaxWidth is required."));
        var maxHeight = int.Parse(_configuration["Image:MaxHeight"] ?? throw new InvalidOperationException("Image:MaxHeight is required."));

        if (width < minWidth || height < minHeight)
        {
            throw new InvalidOperationException("Image resolution is too low.");
        }

        if (width > maxWidth || height > maxHeight)
        {
            throw new InvalidOperationException("Image resolution is too high.");
        }

        var extension = Path.GetExtension(fileName).ToLower();
        var isAnimated = image.Frames.Count > 1;
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
