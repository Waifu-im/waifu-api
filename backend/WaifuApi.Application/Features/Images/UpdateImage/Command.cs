using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Images.UpdateImage;

public record UpdateImageCommand(long Id, string? Source, bool IsNsfw, long? UserId, List<long>? TagIds, long? ArtistId) : ICommand<ImageDto>;

public class UpdateImageCommandHandler : ICommandHandler<UpdateImageCommand, ImageDto>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;
    private readonly IConfiguration _configuration;

    public UpdateImageCommandHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
        _configuration = configuration;
    }

    public async ValueTask<ImageDto> Handle(UpdateImageCommand request, CancellationToken cancellationToken)
    {
        var image = await _context.Images
            .Include(i => i.Tags)
            .FirstOrDefaultAsync(i => i.Id == request.Id, cancellationToken);

        if (image == null)
        {
            throw new KeyNotFoundException($"Image with ID {request.Id} not found.");
        }

        image.Source = request.Source;
        image.IsNsfw = request.IsNsfw;

        if (request.UserId.HasValue)
        {
            // Verify user exists
            var user = await _context.Users.FindAsync(new object[] { request.UserId.Value }, cancellationToken);
            if (user == null)
            {
                throw new KeyNotFoundException($"User with ID {request.UserId.Value} not found.");
            }
            image.UploaderId = request.UserId.Value;
        }

        if (request.TagIds != null)
        {
            image.Tags.Clear();
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
                image.Tags = foundTags;
            }
        }

        if (request.ArtistId.HasValue)
        {
            var artist = await _context.Artists.FindAsync(new object[] { request.ArtistId.Value }, cancellationToken);
            if (artist == null)
            {
                throw new KeyNotFoundException($"Artist with ID {request.ArtistId.Value} not found.");
            }
            image.Artist = artist;
        }
        else
        {
            image.Artist = null;
        }

        await _context.SaveChangesAsync(cancellationToken);

        // Explicitly load related data (Artist) - Tags are already loaded/updated
        await _context.Entry(image).Reference(i => i.Artist).LoadAsync(cancellationToken);

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
            Favorites = 0, // Not calculating favorites for update response
            LikedAt = null
        };
    }

    private static string BitArrayToHex(BitArray bits)
    {
        var bytes = new byte[(bits.Length + 7) / 8];
        bits.CopyTo(bytes, 0);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }
}
