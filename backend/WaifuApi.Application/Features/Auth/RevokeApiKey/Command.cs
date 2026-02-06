using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Auth.RevokeApiKey;

public record RevokeApiKeyCommand(long UserId, long KeyId) : ICommand;

public class RevokeApiKeyCommandHandler : ICommandHandler<RevokeApiKeyCommand>
{
    private readonly IWaifuDbContext _context;

    public RevokeApiKeyCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(RevokeApiKeyCommand request, CancellationToken cancellationToken)
    {
        var key = await _context.ApiKeys
            .FirstOrDefaultAsync(k => k.Id == request.KeyId && k.UserId == request.UserId, cancellationToken);

        if (key != null)
        {
            _context.ApiKeys.Remove(key);
            await _context.SaveChangesAsync(cancellationToken);
        }
        return Unit.Value;
    }
}
