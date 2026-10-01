using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Banner.GetBannerSettings;

/// <summary>Returns the full banner configuration (defaults when it has never been configured).</summary>
public record GetBannerSettingsQuery : IQuery<BannerSettingsDto>;

public class GetBannerSettingsQueryHandler : IQueryHandler<GetBannerSettingsQuery, BannerSettingsDto>
{
    private readonly IWaifuDbContext _context;

    public GetBannerSettingsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<BannerSettingsDto> Handle(GetBannerSettingsQuery request, CancellationToken cancellationToken)
    {
        var banner = await _context.SiteBanners.AsNoTracking()
            .Include(b => b.UpdatedBy)
            .FirstOrDefaultAsync(b => b.Id == SiteBanner.SingletonId, cancellationToken);

        return BannerMapping.ToSettingsDto(banner);
    }
}

public static class BannerMapping
{
    public static BannerSettingsDto ToSettingsDto(SiteBanner? banner)
    {
        if (banner == null)
        {
            return new BannerSettingsDto();
        }

        return new BannerSettingsDto
        {
            Message = banner.Message,
            Variant = banner.Variant,
            IsEnabled = banner.IsEnabled,
            UpdatedAt = banner.UpdatedAt,
            UpdatedBy = banner.UpdatedBy == null ? null : new UserMinimalDto
            {
                Id = banner.UpdatedBy.Id,
                Name = banner.UpdatedBy.Name,
                AvatarUrl = banner.UpdatedBy.AvatarUrl
            }
        };
    }
}
