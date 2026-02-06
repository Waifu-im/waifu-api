using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Common.Models; // Pour TagDto
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Tags.CreateTag;

public record CreateTagCommand(string Name, string Description, string? Slug) : ICommand<TagDto>;

public class CreateTagCommandHandler : ICommandHandler<CreateTagCommand, TagDto>
{
    private readonly IWaifuDbContext _context;
    private readonly IConfiguration _configuration;

    public CreateTagCommandHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _configuration = configuration;
    }

    public async ValueTask<TagDto> Handle(CreateTagCommand request, CancellationToken cancellationToken)
    {
        var trimmedName = request.Name.Trim();
        
        // Génération automatique du slug si non fourni
        var slug = !string.IsNullOrWhiteSpace(request.Slug) 
            ? request.Slug.Trim().ToSlug() 
            : trimmedName.ToSlug();

        var existingTag = await _context.Tags
            .Select(t => new { t.Name, t.Slug })
            .FirstOrDefaultAsync(t => t.Slug == slug || t.Name.ToLower() == trimmedName.ToLower(), cancellationToken);

        if (existingTag != null)
        {
            if (existingTag.Slug == slug)
            {
                throw new ConflictException($"Tag with slug '{slug}' already exists.");
            }

            throw new ConflictException($"Tag with name '{trimmedName}' already exists.");
        }

        var requireReview = bool.Parse(_configuration["Moderation:RequireTagReview"] ?? "true");
        
        var tag = new Tag
        {
            Name = trimmedName,
            Slug = slug,
            Description = request.Description?.Trim() ?? string.Empty,
            ReviewStatus = requireReview ? ReviewStatus.Pending : ReviewStatus.Accepted
        };

        _context.Tags.Add(tag);
        await _context.SaveChangesAsync(cancellationToken);

        return new TagDto
        {
            Id = tag.Id,
            Name = tag.Name,
            Slug = tag.Slug,
            Description = tag.Description,
            ReviewStatus = tag.ReviewStatus
        };
    }
}