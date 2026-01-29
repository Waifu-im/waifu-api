using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Tags.GetTagBySlug;

public record GetTagBySlugQuery(string Slug) : IQuery<TagDto>;

public class GetTagBySlugQueryHandler : IQueryHandler<GetTagBySlugQuery, TagDto>
{
    private readonly IWaifuDbContext _context;

    public GetTagBySlugQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<TagDto> Handle(GetTagBySlugQuery request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags
            .AsNoTracking()
            .FirstOrDefaultAsync(t => t.Slug == request.Slug, cancellationToken);

        if (tag == null)
        {
            throw new KeyNotFoundException($"Tag with slug '{request.Slug}' not found.");
        }

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