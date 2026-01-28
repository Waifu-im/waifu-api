using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.Tags.GetPendingTags;

public record GetPendingTagsQuery : IQuery<List<Tag>>;

public class GetPendingTagsQueryHandler : IQueryHandler<GetPendingTagsQuery, List<Tag>>
{
    private readonly IWaifuDbContext _context;

    public GetPendingTagsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<Tag>> Handle(GetPendingTagsQuery request, CancellationToken cancellationToken)
    {
        return await _context.Tags
            .Where(t => t.ReviewStatus == ReviewStatus.Pending)
            .ToListAsync(cancellationToken);
    }
}
