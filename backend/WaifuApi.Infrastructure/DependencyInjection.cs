using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using WaifuApi.Application.Interfaces;
using WaifuApi.Infrastructure.Persistence;
using WaifuApi.Infrastructure.Services;

namespace WaifuApi.Infrastructure;

public static class DependencyInjection
{
    public static IServiceCollection AddInfrastructure(this IServiceCollection services, IConfiguration configuration)
    {
        services.AddDbContext<WaifuDbContext>(options =>
            options.UseNpgsql(configuration.GetConnectionString("DefaultConnection")));

        services.AddScoped<IWaifuDbContext>(provider => provider.GetRequiredService<WaifuDbContext>());

        services.AddScoped<ITokenService, TokenService>();
        services.AddScoped<IDiscordService, DiscordService>();
        services.AddScoped<IStorageService, S3Service>();
        services.AddScoped<IImageProcessingService, ImageProcessingService>();
        
        services.AddHttpClient();

        return services;
    }
}
