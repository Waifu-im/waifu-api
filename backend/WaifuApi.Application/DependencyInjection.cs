using System.Reflection;
using FluentValidation;
using Mediator;
using Microsoft.Extensions.DependencyInjection;
using WaifuApi.Application.Common.Behaviors;
using WaifuApi.Application.Common.Services;
using WaifuApi.Application.Interfaces;

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

        // Register the ValidationBehavior for all requests
        services.AddScoped(typeof(IPipelineBehavior<,>), typeof(ValidationBehavior<,>));

        // Shared review rules (review/bypass) used by both content creation and edit requests.
        services.AddSingleton<IReviewPolicy, ReviewPolicy>();
        services.AddScoped<IEditApplier, EditApplier>();
        services.AddScoped<IEditPreparer, EditPreparer>();
        services.AddScoped<IReviewTargetService, ReviewTargetService>();

        return services;
    }
}
