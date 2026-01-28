using System;
using System.Collections;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Images.UpdateImage;

public record UpdateImageCommand(long Id, string? Source, bool? IsNsfw, long? UserId) : ICommand<ImageDto>;

public class UpdateImageCommandHandler : ICommandHandler<UpdateImageCommand, ImageDto>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;

    public UpdateImageCommandHandler(IWaifuDbContext context, Microsoft.Extensions.Configuration.IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new System.InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public async ValueTask<ImageDto> Handle(UpdateImageCommand request, CancellationToken cancellationToken)
    {
        var image = await _context.Images.FindAsync(new object[] { request.Id }, cancellationToken);

        if (image == null)
        {
            throw new KeyNotFoundException($"Image with ID {request.Id} not found.");
        }

        if (request.Source != null)
        {
            image.Source = request.Source;
        }

        if (request.IsNsfw.HasValue)
        {
            image.IsNsfw = request.IsNsfw.Value;
        }

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

        await _context.SaveChangesAsync(cancellationToken);

        // Explicitly load related data
        await _context.Entry(image).Collection(i => i.Tags).LoadAsync(cancellationToken);
        await _context.Entry(image).Reference(i => i.Artist).LoadAsync(cancellationToken);

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
