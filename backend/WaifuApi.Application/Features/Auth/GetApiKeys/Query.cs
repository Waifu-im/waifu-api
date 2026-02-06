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
        var keys = await _context.ApiKeys
            .Where(k => k.UserId == request.UserId)
            .OrderByDescending(k => k.CreatedAt)
            .ToListAsync(cancellationToken);

        return keys.Select(k => new ApiKeyDto
        {
            Id = k.Id,
            Key = MaskKey(k.Key),
            Description = k.Description,
            CreatedAt = k.CreatedAt,
            LastUsedAt = k.LastUsedAt,
            ExpirationDate = k.ExpirationDate,
            UserId = k.UserId
        }).ToList();
    }

    private static string MaskKey(string key)
    {
        if (string.IsNullOrEmpty(key) || key.Length <= 8)
            return "********";
        return key[..8] + "...";
    }
}
