using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Banner.GetActiveBanner;

/// <summary>Returns the banner shown to visitors, or null when it is disabled or empty.</summary>
public record GetActiveBannerQuery : IQuery<BannerDto?>;

public class GetActiveBannerQueryHandler : IQueryHandler<GetActiveBannerQuery, BannerDto?>
{
    private readonly IWaifuDbContext _context;

    public GetActiveBannerQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<BannerDto?> Handle(GetActiveBannerQuery request, CancellationToken cancellationToken)
    {
        var banner = await _context.SiteBanners.AsNoTracking()
            .FirstOrDefaultAsync(b => b.Id == SiteBanner.SingletonId, cancellationToken);

        if (banner == null || !banner.IsEnabled || string.IsNullOrWhiteSpace(banner.Message))
        {
            return null;
        }

        return new BannerDto
        {
            Message = banner.Message,
            Variant = banner.Variant,
            UpdatedAt = banner.UpdatedAt
        };
    }
}
