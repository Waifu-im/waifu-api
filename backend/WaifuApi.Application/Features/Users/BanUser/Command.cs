using Mediator;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Users.BanUser;

public record BanUserCommand(long UserId, bool IsBlacklisted) : ICommand;

public class BanUserCommandHandler : ICommandHandler<BanUserCommand>
{
    private readonly IWaifuDbContext _context;

    public BanUserCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(BanUserCommand request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.FindAsync(new object[] { request.UserId }, cancellationToken);
        if (user != null)
        {
            user.IsBlacklisted = request.IsBlacklisted;
            await _context.SaveChangesAsync(cancellationToken);
        }
        return Unit.Value;
    }
}
