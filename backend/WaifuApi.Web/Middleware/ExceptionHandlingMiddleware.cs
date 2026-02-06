using System;
using System.Collections.Generic;
using System.Net;
using System.Text.Json;
using System.Threading.Tasks;
using FluentValidation;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Web.Models;

namespace WaifuApi.Web.Middleware;

public class ExceptionHandlingMiddleware
{
    private readonly RequestDelegate _next;
    private readonly ILogger<ExceptionHandlingMiddleware> _logger;

    public ExceptionHandlingMiddleware(RequestDelegate next, ILogger<ExceptionHandlingMiddleware> logger)
    {
        _next = next;
        _logger = logger;
    }

    public async Task Invoke(HttpContext context)
    {
        try
        {
            await _next(context);
        }
        catch (Exception ex)
        {
            if (IsExpectedException(ex))
            {
                _logger.LogWarning("A request error occurred: {Message}", ex.Message);
            }
            else
            {
                _logger.LogError(ex, "An unhandled exception occurred while processing the request.");
            }

            await HandleExceptionAsync(context, ex);
        }
    }

    private static bool IsExpectedException(Exception ex)
    {
        return ex is ValidationException
            || ex is UnauthorizedAccessException
            || ex is ForbiddenException
            || ex is KeyNotFoundException
            || ex is ConflictException
            || ex is ArgumentException
            || ex is InvalidOperationException;
    }

    private static Task HandleExceptionAsync(HttpContext context, Exception exception)
    {
        if (context.Response.HasStarted)
        {
            return Task.CompletedTask;
        }

        var statusCode = HttpStatusCode.InternalServerError;
        var traceId = System.Diagnostics.Activity.Current?.Id ?? context.TraceIdentifier;
        object response;

        switch (exception)
        {
            case ValidationException validationException:
                statusCode = HttpStatusCode.BadRequest;
                var errors = new Dictionary<string, string[]>();
                foreach (var error in validationException.Errors)
                {
                    if (errors.ContainsKey(error.PropertyName))
                    {
                        var list = new List<string>(errors[error.PropertyName]);
                        list.Add(error.ErrorMessage);
                        errors[error.PropertyName] = list.ToArray();
                    }
                    else
                    {
                        errors.Add(error.PropertyName, new[] { error.ErrorMessage });
                    }
                }
                response = new ValidationErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.5.1",
                    Title = "One or more validation errors occurred.",
                    Status = (int)statusCode,
                    Detail = "See errors property for details.",
                    Instance = context.Request.Path,
                    TraceId = traceId,
                    Errors = errors
                };
                break;
            case UnauthorizedAccessException:
                statusCode = HttpStatusCode.Unauthorized;
                response = new ErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.5.2",
                    Title = "Unauthorized",
                    Status = (int)statusCode,
                    Detail = exception.Message,
                    Instance = context.Request.Path,
                    TraceId = traceId
                };
                break;
            case ForbiddenException:
                statusCode = HttpStatusCode.Forbidden;
                response = new ErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.5.4",
                    Title = "Forbidden",
                    Status = (int)statusCode,
                    Detail = exception.Message,
                    Instance = context.Request.Path,
                    TraceId = traceId
                };
                break;
            case KeyNotFoundException:
                statusCode = HttpStatusCode.NotFound;
                response = new ErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.5.5",
                    Title = "Resource Not Found",
                    Status = (int)statusCode,
                    Detail = exception.Message,
                    Instance = context.Request.Path,
                    TraceId = traceId
                };
                break;
            case ConflictException:
                statusCode = HttpStatusCode.Conflict;
                response = new ErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.5.10",
                    Title = "Conflict",
                    Status = (int)statusCode,
                    Detail = exception.Message,
                    Instance = context.Request.Path,
                    TraceId = traceId
                };
                break;
            case ArgumentException:
                statusCode = HttpStatusCode.BadRequest;
                response = new ErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.5.1",
                    Title = "Bad Request",
                    Status = (int)statusCode,
                    Detail = exception.Message,
                    Instance = context.Request.Path,
                    TraceId = traceId
                };
                break;
            case InvalidOperationException:
                statusCode = HttpStatusCode.BadRequest;
                response = new ErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.5.1",
                    Title = "Invalid Operation",
                    Status = (int)statusCode,
                    Detail = exception.Message,
                    Instance = context.Request.Path,
                    TraceId = traceId
                };
                break;
            default:
                response = new ErrorResponse
                {
                    Type = "https://tools.ietf.org/html/rfc9110#section-15.6.1",
                    Title = "An error occurred while processing your request.",
                    Status = (int)statusCode,
                    Detail = exception.Message,
                    Instance = context.Request.Path,
                    TraceId = traceId
                };
                break;
        }

        context.Response.ContentType = "application/problem+json";
        context.Response.StatusCode = (int)statusCode;

        var json = JsonSerializer.Serialize(response, new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase
        });
        return context.Response.WriteAsync(json);
    }
}
