using System;
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

namespace WaifuApi.Application.Features.Images.UpdateImage;

public record UpdateImageCommand(
    long Id, 
    string? Source, 
    bool IsNsfw, 
    long? UserId, 
    List<long>? TagIds, 
    List<long>? ArtistIds
) : ICommand<ImageDto>;

public class UpdateImageCommandHandler : ICommandHandler<UpdateImageCommand, ImageDto>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;

    public UpdateImageCommandHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public async ValueTask<ImageDto> Handle(UpdateImageCommand request, CancellationToken cancellationToken)
    {
        var image = await _context.Images
            .Include(i => i.Tags)
            .Include(i => i.Artists)
            .FirstOrDefaultAsync(i => i.Id == request.Id, cancellationToken);

        if (image == null)
        {
            throw new KeyNotFoundException($"Image with ID {request.Id} not found.");
        }

        // Clean string input
        image.Source = request.Source.ToNullIfEmpty();
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
            // Update tags only if list provided
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

        if (request.ArtistIds != null)
        {
            image.Artists.Clear();
            if (request.ArtistIds.Any())
            {
                var foundArtists = await _context.Artists
                    .Where(a => request.ArtistIds.Contains(a.Id))
                    .ToListAsync(cancellationToken);

                if (foundArtists.Count != request.ArtistIds.Count)
                {
                    var foundIds = foundArtists.Select(a => a.Id).ToList();
                    var missingIds = request.ArtistIds.Except(foundIds).ToList();
                    throw new KeyNotFoundException($"Artists with IDs {string.Join(", ", missingIds)} not found.");
                }
                image.Artists = foundArtists;
            }
        }

        await _context.SaveChangesAsync(cancellationToken);

        return new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayHelper.ToHex(image.PerceptualHash),
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artists = image.Artists,
            UploaderId = image.UploaderId,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, image.Id, image.Extension),
            // Map Tags to DTO
            Tags = image.Tags.Select(t => new TagDto
            {
                Id = t.Id,
                Name = t.Name,
                Slug = t.Slug,
                Description = t.Description,
                ReviewStatus = t.ReviewStatus
            }).ToList(),
            Favorites = 0, // Not relevant for update response
            LikedAt = null
        };
    }
}