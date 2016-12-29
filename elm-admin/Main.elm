module Main exposing (..)

import Html exposing (Html, text, select, div, label, option, input, button)
import Html.App as App
import Html.Attributes exposing (for, id, value, type', style, selected, placeholder)
import Html.Events exposing (onClick, onInput, on, targetValue, keyCode)
import Http
import Json.Decode as JSD
import Json.Encode as JSE
import Task


main : Program Never
main =
    App.program
        { init = init
        , view = view
        , update = update
        , subscriptions = subscriptions
        }



-- INIT / MODEL


type alias Model =
    { beforeDate : String
    , status : String
    }


init : ( Model, Cmd Msg )
init =
    ( { beforeDate = ""
      , status = ""
      }
    , Cmd.none
    )



-- UPDATE


type Msg
    = Entry String
    | Submit
    | PurgeReqOK String
    | PurgeReqFail Http.Error


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Entry newDate ->
            ( { model | beforeDate = newDate }, Cmd.none )

        Submit ->
            ( { model | status = "Purge request sent" }, sendPurgeRequest model.beforeDate )

        PurgeReqOK result ->
            ( { model | status = result }, Cmd.none )

        PurgeReqFail error ->
            ( { model | status = (toString error) }, Cmd.none )



-- VIEW


view : Model -> Html Msg
view model =
    div []
        [ input [ placeholder "2015-01-01", onInput Entry ] []
        , button [ onClick Submit ] [ text "Purge" ]
        , div [ id "status" ] [ text model.status ]
        ]



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- LOGIC


sendPurgeRequest : String -> Cmd Msg
sendPurgeRequest dateStr =
    let
        request =
            Http.post JSD.string
                "/purge/"
            <|
                Http.string <|
                    JSE.encode 0 <|
                        JSE.object [ ( "before", JSE.string dateStr ) ]
    in
        Task.perform PurgeReqFail PurgeReqOK request
