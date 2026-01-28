using System;
using System.Collections;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Pgvector.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
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
    private const int HammingDistanceThreshold = 4;

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
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public async ValueTask<ImageDto> Handle(UploadImageCommand request, CancellationToken cancellationToken)
    {
        var metadata = await _imageProcessingService.ProcessAsync(request.FileStream, request.FileName);

        var targetHash = HexToBitArray(metadata.PerceptualHash);

        // Use pgvector HammingDistance
        var duplicate = await _context.Images
            .Where(i => i.PerceptualHash.HammingDistance(targetHash) <= HammingDistanceThreshold)
            .Select(i => new { i.Id })
            .FirstOrDefaultAsync(cancellationToken);

        if (duplicate != null)
        {
            throw new ConflictException($"Duplicate image found. ID: {duplicate.Id}");
        }

        Artist? artist = null;
        if (!string.IsNullOrEmpty(request.ArtistName))
        {
            artist = await _context.Artists.FirstOrDefaultAsync(a => a.Name == request.ArtistName, cancellationToken);
            if (artist == null)
            {
                var requireReview = bool.Parse(_configuration["Moderation:RequireArtistReview"] ?? throw new InvalidOperationException("Moderation:RequireArtistReview is required."));
                artist = new Artist
                {
                    Name = request.ArtistName,
                    ReviewStatus = requireReview ? ReviewStatus.Pending : ReviewStatus.Accepted
                };
                _context.Artists.Add(artist);
            }
        }

        var tags = new List<Tag>();
        var requireTagReview = bool.Parse(_configuration["Moderation:RequireTagReview"] ?? throw new InvalidOperationException("Moderation:RequireTagReview is required."));
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

        var requireImageReview = bool.Parse(_configuration["Moderation:RequireImageReview"] ?? throw new InvalidOperationException("Moderation:RequireImageReview is required."));
        var image = new Image
        {
            PerceptualHash = targetHash,
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

        return new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayToHex(image.PerceptualHash),
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

    private static BitArray HexToBitArray(string hex)
    {
        var bytes = Convert.FromHexString(hex);
        return new BitArray(bytes);
    }

    private static string BitArrayToHex(BitArray bits)
    {
        var bytes = new byte[(bits.Length + 7) / 8];
        bits.CopyTo(bytes, 0);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }
}
