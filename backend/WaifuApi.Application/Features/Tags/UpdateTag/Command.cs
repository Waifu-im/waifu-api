using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Tags.UpdateTag;

public record UpdateTagCommand(long Id, string Name, string Description, string? Slug) : ICommand<TagDto>;

public class UpdateTagCommandHandler : ICommandHandler<UpdateTagCommand, TagDto>
{
    private readonly IWaifuDbContext _context;

    public UpdateTagCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<TagDto> Handle(UpdateTagCommand request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags.FindAsync(new object[] { request.Id }, cancellationToken);

        if (tag == null)
        {
            throw new KeyNotFoundException($"Tag with ID {request.Id} not found.");
        }

        // Check for name conflict
        var nameExists = await _context.Tags.AnyAsync(t => t.Id != request.Id && t.Name.ToLower() == request.Name.ToLower(), cancellationToken);
        if (nameExists)
        {
            throw new ConflictException($"Tag with name '{request.Name}' already exists.");
        }

        tag.Name = request.Name;
        tag.Description = request.Description;

        if (!string.IsNullOrWhiteSpace(request.Slug))
        {
            var newSlug = request.Slug.ToSlug();
            if (tag.Slug != newSlug)
            {
                var exists = await _context.Tags.AnyAsync(t => t.Slug == newSlug, cancellationToken);
                if (exists) throw new ConflictException($"Tag with slug '{newSlug}' already exists.");
                tag.Slug = newSlug;
            }
        }
        
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