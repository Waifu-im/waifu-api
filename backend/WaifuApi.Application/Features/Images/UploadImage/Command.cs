using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Images.UploadImage;

public record UploadImageCommand(
    long UserId,
    Stream FileStream,
    string FileName,
    string ContentType,
    string? ArtistName,
    List<string> Tags,
    string? Source,
    bool IsNsfw
) : ICommand<ImageDto>;

public class UploadImageCommandHandler : ICommandHandler<UploadImageCommand, ImageDto>
{
    private readonly IWaifuDbContext _context;
    private readonly IStorageService _storageService;
    private readonly IImageProcessingService _imageProcessingService;
    private readonly IConfiguration _configuration;
    private readonly string _cdnBaseUrl;

    public UploadImageCommandHandler(
        IWaifuDbContext context,
        IStorageService storageService,
        IImageProcessingService imageProcessingService,
        IConfiguration configuration)
    {
        _context = context;
        _storageService = storageService;
        _imageProcessingService = imageProcessingService;
        _configuration = configuration;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? "https://cdn.waifu.im";
    }

    public async ValueTask<ImageDto> Handle(UploadImageCommand request, CancellationToken cancellationToken)
    {
        // 1. Process Image
        var metadata = await _imageProcessingService.ProcessAsync(request.FileStream, request.FileName);

        // 2. Check Duplicates
        var targetHash = ulong.Parse(metadata.PerceptualHash, System.Globalization.NumberStyles.HexNumber);
        
        var existingHashes = await _context.Images
            .AsNoTracking()
            .Select(i => new { i.Id, i.PerceptualHash })
            .ToListAsync(cancellationToken);

        foreach (var item in existingHashes)
        {
            if (ulong.TryParse(item.PerceptualHash, System.Globalization.NumberStyles.HexNumber, null, out var existingHash))
            {
                if (System.Numerics.BitOperations.PopCount(targetHash ^ existingHash) <= 4)
                {
                    throw new InvalidOperationException($"Duplicate image found. ID: {item.Id}");
                }
            }
        }

        // 3. Handle Artist
        Artist? artist = null;
        if (!string.IsNullOrEmpty(request.ArtistName))
        {
            artist = await _context.Artists.FirstOrDefaultAsync(a => a.Name == request.ArtistName, cancellationToken);
            if (artist == null)
            {
                var requireReview = _configuration.GetValue<bool>("Moderation:RequireArtistReview");
                artist = new Artist
                {
                    Name = request.ArtistName,
                    ReviewStatus = requireReview ? ReviewStatus.Pending : ReviewStatus.Accepted
                };
                _context.Artists.Add(artist);
            }
        }

        // 4. Handle Tags
        var tags = new List<Tag>();
        var requireTagReview = _configuration.GetValue<bool>("Moderation:RequireTagReview");
        foreach (var tagName in request.Tags)
        {
            var tag = await _context.Tags.FirstOrDefaultAsync(t => t.Name == tagName, cancellationToken);
            if (tag == null)
            {
                tag = new Tag
                {
                    Name = tagName,
                    Description = "",
                    ReviewStatus = requireTagReview ? ReviewStatus.Pending : ReviewStatus.Accepted
                };
                _context.Tags.Add(tag);
            }
            tags.Add(tag);
        }

        // 5. Create Image Entity
        var requireImageReview = _configuration.GetValue<bool>("Moderation:RequireImageReview");
        var image = new Image
        {
            PerceptualHash = metadata.PerceptualHash,
            Extension = metadata.Extension,
            DominantColor = metadata.DominantColor,
            Source = request.Source,
            Artist = artist,
            UploaderId = request.UserId,
            UploadedAt = DateTime.UtcNow,
            IsNsfw = request.IsNsfw,
            IsAnimated = metadata.IsAnimated,
            Width = metadata.Width,
            Height = metadata.Height,
            ByteSize = metadata.ByteSize,
            ReviewStatus = requireImageReview ? ReviewStatus.Pending : ReviewStatus.Accepted,
            Tags = tags
        };

        _context.Images.Add(image);
        await _context.SaveChangesAsync(cancellationToken);

        // 6. Upload to S3
        try
        {
            request.FileStream.Position = 0;
            var s3FileName = $"{image.Id}{metadata.Extension}";
            await _storageService.UploadAsync(request.FileStream, s3FileName, request.ContentType);
        }
        catch
        {
            _context.Images.Remove(image);
            await _context.SaveChangesAsync(cancellationToken);
            throw;
        }

        // 7. Return DTO
        return new ImageDto
        {
            Id = image.Id,
            Signature = image.PerceptualHash,
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artist = image.Artist,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = $"{_cdnBaseUrl}/{image.Id}{image.Extension}",
            Tags = image.Tags,
            Favorites = 0,
            LikedAt = null
        };
    }
}
