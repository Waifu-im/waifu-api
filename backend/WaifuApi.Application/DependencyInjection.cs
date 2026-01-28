using System.Reflection;
using FluentValidation;
using Microsoft.Extensions.DependencyInjection;
using WaifuApi.Application.Common.Behaviors;

namespace WaifuApi.Application;

public static class DependencyInjection
{
    public static IServiceCollection AddApplication(this IServiceCollection services)
    {
        services.AddMediator(options =>
        {
            options.ServiceLifetime = ServiceLifetime.Scoped;
        });
        
        services.AddValidatorsFromAssembly(Assembly.GetExecutingAssembly());
        
        // Register behaviors manually or via Mediator options if supported differently
        // Mediator supports pipeline behaviors via partial class extension or interface implementation
        // For now, let's stick to basic registration. 
        // Mediator.SourceGenerator handles pipelines differently (IPipelineBehavior).

        return services;
    }
}
