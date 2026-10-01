using FluentValidation;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Banner.GetActiveBanner;
using WaifuApi.Application.Features.Banner.GetBannerSettings;
using WaifuApi.Application.Features.Banner.UpdateBanner;
using WaifuApi.Domain.Enums;
using WaifuApi.Tests.Infrastructure;
using Xunit;

namespace WaifuApi.Tests;

public class BannerTests
{
    [Fact]
    public async Task NeverConfigured_NoActiveBanner_DefaultSettings()
    {
        using var harness = new TestHarness();

        Assert.Null(await harness.QueryAsync<BannerDto?>(new GetActiveBannerQuery()));

        var settings = await harness.QueryAsync<BannerSettingsDto>(new GetBannerSettingsQuery());
        Assert.False(settings.IsEnabled);
        Assert.Equal(string.Empty, settings.Message);
        Assert.Null(settings.UpdatedAt);
    }

    [Fact]
    public async Task Update_CreatesThenReplacesSingleBanner()
    {
        using var harness = new TestHarness();
        var modId = await harness.SeedUserAsync(Role.Moderator);

        var created = await harness.SendAsync<BannerSettingsDto>(new UpdateBannerCommand(modId, "  Maintenance tonight  ", BannerVariant.Warning, true));
        Assert.Equal("Maintenance tonight", created.Message);
        Assert.Equal(modId, created.UpdatedBy?.Id);

        var active = await harness.QueryAsync<BannerDto?>(new GetActiveBannerQuery());
        Assert.NotNull(active);
        Assert.Equal(BannerVariant.Warning, active!.Variant);

        await harness.SendAsync<BannerSettingsDto>(new UpdateBannerCommand(modId, "Back online", BannerVariant.Success, true));
        Assert.Equal(1, await harness.ReadAsync(db => Task.FromResult(db.SiteBanners.Count())));
        Assert.Equal("Back online", (await harness.QueryAsync<BannerDto?>(new GetActiveBannerQuery()))!.Message);
    }

    [Fact]
    public async Task Disabled_KeepsMessageButHidesBanner()
    {
        using var harness = new TestHarness();
        var modId = await harness.SeedUserAsync(Role.Moderator);

        await harness.SendAsync<BannerSettingsDto>(new UpdateBannerCommand(modId, "Draft", BannerVariant.Info, false));

        Assert.Null(await harness.QueryAsync<BannerDto?>(new GetActiveBannerQuery()));
        Assert.Equal("Draft", (await harness.QueryAsync<BannerSettingsDto>(new GetBannerSettingsQuery())).Message);
    }

    [Fact]
    public async Task Enabled_RequiresMessage_AndEnforcesMaxLength()
    {
        using var harness = new TestHarness();
        var modId = await harness.SeedUserAsync(Role.Moderator);

        await Assert.ThrowsAsync<ValidationException>(() => harness.SendAsync<BannerSettingsDto>(
            new UpdateBannerCommand(modId, "   ", BannerVariant.Info, true)));
        await Assert.ThrowsAsync<ValidationException>(() => harness.SendAsync<BannerSettingsDto>(
            new UpdateBannerCommand(modId, new string('a', UpdateBannerCommandValidator.MaxMessageLength + 1), BannerVariant.Info, false)));
    }
}
