using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Tags.GetTagByName;

public record GetTagByNameQuery(string Name) : IQuery<TagDto>;

public class GetTagByNameQueryHandler : IQueryHandler<GetTagByNameQuery, TagDto>
{
    private readonly IWaifuDbContext _context;

    public GetTagByNameQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<TagDto> Handle(GetTagByNameQuery request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags
            .AsNoTracking()
            .FirstOrDefaultAsync(t => t.Name == request.Name, cancellationToken);

        if (tag == null)
        {
            throw new KeyNotFoundException($"Tag with name '{request.Name}' not found.");
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