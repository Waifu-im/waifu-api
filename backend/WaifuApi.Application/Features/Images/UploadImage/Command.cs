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
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Images.UploadImage;

public record UploadImageCommand(
    long UserId,
    Stream FileStream,
    string FileName,
    string ContentType,
    long? ArtistId,
    List<long> TagIds,
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
        if (request.ArtistId.HasValue)
        {
            artist = await _context.Artists.FindAsync(new object[] { request.ArtistId.Value }, cancellationToken);
            if (artist == null)
            {
                throw new KeyNotFoundException($"Artist with ID {request.ArtistId.Value} not found.");
            }
        }

        var tags = new List<Tag>();
        if (request.TagIds.Any())
        {
            var foundTags = await _context.Tags
                .Where(t => request.TagIds.Contains(t.Id))
                .ToListAsync(cancellationToken);

            if (foundTags.Count != request.TagIds.Count)
            {
                var foundIds = foundTags.Select(t => t.Id).ToList();
                var missingIds = request.TagIds.Except(foundIds).ToList();
                throw new KeyNotFoundException($"Tags with IDs {string.Join(", ", missingIds)} not found.");
            }
            tags = foundTags;
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
            UploaderId = image.UploaderId,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, image.Id, image.Extension),
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
