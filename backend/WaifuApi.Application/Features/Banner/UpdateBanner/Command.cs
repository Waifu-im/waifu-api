using System;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Banner.GetBannerSettings;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Banner.UpdateBanner;

/// <summary>Creates or replaces the site banner. Permission is checked by the caller (configurable min role).</summary>
public record UpdateBannerCommand(long UserId, string Message, BannerVariant Variant, bool IsEnabled) : ICommand<BannerSettingsDto>;

public class UpdateBannerCommandHandler : ICommandHandler<UpdateBannerCommand, BannerSettingsDto>
{
    private readonly IWaifuDbContext _context;

    public UpdateBannerCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<BannerSettingsDto> Handle(UpdateBannerCommand request, CancellationToken cancellationToken)
    {
        var banner = await _context.SiteBanners
            .FirstOrDefaultAsync(b => b.Id == SiteBanner.SingletonId, cancellationToken);

        if (banner == null)
        {
            banner = new SiteBanner();
            _context.SiteBanners.Add(banner);
        }

        banner.Message = request.Message.Trim();
        banner.Variant = request.Variant;
        banner.IsEnabled = request.IsEnabled;
        banner.UpdatedAt = DateTime.UtcNow;
        banner.UpdatedById = request.UserId;

        await _context.SaveChangesAsync(cancellationToken);

        banner.UpdatedBy = await _context.Users.FindAsync(new object[] { request.UserId }, cancellationToken);
        return BannerMapping.ToSettingsDto(banner);
    }
}
