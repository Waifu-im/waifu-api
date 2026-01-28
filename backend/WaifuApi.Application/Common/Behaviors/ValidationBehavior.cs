using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using FluentValidation;
using Mediator;

namespace WaifuApi.Application.Common.Behaviors;

public class ValidationBehavior<TMessage, TResponse> : IPipelineBehavior<TMessage, TResponse>
    where TMessage : IMessage
{
    private readonly IEnumerable<IValidator<TMessage>> _validators;

    public ValidationBehavior(IEnumerable<IValidator<TMessage>> validators)
    {
        _validators = validators;
    }

    public ValueTask<TResponse> Handle(TMessage message, MessageHandlerDelegate<TMessage, TResponse> next, CancellationToken cancellationToken)
    {
        if (!_validators.Any())
        {
            return next(message, cancellationToken);
        }

        return HandleInternal(message, next, cancellationToken);
    }

    private async ValueTask<TResponse> HandleInternal(TMessage message, MessageHandlerDelegate<TMessage, TResponse> next, CancellationToken cancellationToken)
    {
        var context = new ValidationContext<TMessage>(message);

        var validationResults = await Task.WhenAll(
            _validators.Select(v => v.ValidateAsync(context, cancellationToken)));

        var failures = validationResults
            .SelectMany(r => r.Errors)
            .Where(f => f != null)
            .ToList();

        if (failures.Count != 0)
        {
            throw new ValidationException(failures);
        }

        return await next(message, cancellationToken);
    }
}
