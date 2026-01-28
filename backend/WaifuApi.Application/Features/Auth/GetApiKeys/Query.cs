using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Auth.GetApiKeys;

public record GetApiKeysQuery(long UserId) : IQuery<List<ApiKeyDto>>;

public class GetApiKeysQueryHandler : IQueryHandler<GetApiKeysQuery, List<ApiKeyDto>>
{
    private readonly IWaifuDbContext _context;

    public GetApiKeysQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<ApiKeyDto>> Handle(GetApiKeysQuery request, CancellationToken cancellationToken)
    {
        return await _context.ApiKeys
            .Where(k => k.UserId == request.UserId)
            .OrderByDescending(k => k.CreatedAt)
            .Select(k => new ApiKeyDto
            {
                Id = k.Id,
                Key = k.Key,
                Description = k.Description,
                CreatedAt = k.CreatedAt,
                LastUsedAt = k.LastUsedAt,
                ExpirationDate = k.ExpirationDate,
                UserId = k.UserId
            })
            .ToListAsync(cancellationToken);
    }
}
