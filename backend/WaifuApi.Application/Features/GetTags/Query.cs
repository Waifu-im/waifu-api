using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.GetTags;

public class GetTagsQuery : IQuery<List<Tag>>
{
}

public class GetTagsQueryHandler : IQueryHandler<GetTagsQuery, List<Tag>>
{
    private readonly IWaifuDbContext _context;

    public GetTagsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<Tag>> Handle(GetTagsQuery request, CancellationToken cancellationToken)
    {
        return await _context.Tags.ToListAsync(cancellationToken);
    }
}
