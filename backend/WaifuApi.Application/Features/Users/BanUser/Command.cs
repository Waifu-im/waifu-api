using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Users.BanUser;

public record BanUserCommand(long UserId, bool IsBlacklisted) : ICommand<User>;

public class BanUserCommandHandler : ICommandHandler<BanUserCommand, User>
{
    private readonly IWaifuDbContext _context;

    public BanUserCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<User> Handle(BanUserCommand request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.FindAsync(new object[] { request.UserId }, cancellationToken);
        if (user == null)
        {
            throw new KeyNotFoundException($"User with ID {request.UserId} not found.");
        }

        user.IsBlacklisted = request.IsBlacklisted;
        await _context.SaveChangesAsync(cancellationToken);
        
        return user;
    }
}
