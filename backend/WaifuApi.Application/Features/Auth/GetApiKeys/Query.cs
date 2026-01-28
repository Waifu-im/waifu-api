using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Auth.GetApiKeys;

public record GetApiKeysQuery(long UserId) : IQuery<List<ApiKey>>;

public class GetApiKeysQueryHandler : IQueryHandler<GetApiKeysQuery, List<ApiKey>>
{
    private readonly IWaifuDbContext _context;

    public GetApiKeysQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<ApiKey>> Handle(GetApiKeysQuery request, CancellationToken cancellationToken)
    {
        return await _context.ApiKeys
            .Where(k => k.UserId == request.UserId)
            .OrderByDescending(k => k.CreatedAt)
            .ToListAsync(cancellationToken);
    }
}
