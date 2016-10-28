module Main exposing (..)

import Debug
import Entry
import HistoryView as Hv
import Html exposing (Html, text, select, div, label, option, input)
import Html.App as App
import Html.Attributes exposing (class, for, id, value, type')
import Html.Events exposing (onClick, onInput, on, targetValue, keyCode)
import Http
import Json.Decode as Json
import String
import Task


main : Program Never
main =
    App.program
        { init =
            List.head resourceList
                |> Maybe.withDefault ( "departures", "" )
                |> fst
                |> init
        , view = view
        , update = update
        , subscriptions = subscriptions
        }



-- INIT / MODEL


type alias Model =
    { resourceType : String
    , resourceId : String
    , entries : List Entry.Model
    , error : String
    , currentModel : Maybe Entry.Model
    , status : String
    , log : Hv.Log
    }


init : String -> ( Model, Cmd Msg )
init initType =
    ( { resourceType = initType
      , resourceId = ""
      , entries = []
      , error = ""
      , currentModel = Nothing
      , status = ""
      , log = Hv.empty
      }
    , Cmd.none
    )



-- UPDATE


type Msg
    = Entry String
    | GetAction
    | ChangeType String
    | ChangeVersion String
    | FetchFail Http.Error
    | FetchSucceeded (List Entry.Model)
    | KeyPress Int
    | HistoryMsg Hv.Msg


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Entry newId ->
            ( { model | resourceId = newId, status = "", error = "" }, Cmd.none )

        GetAction ->
            updateGetAction model

        ChangeType newType ->
            let
                newmodel =
                    { model | resourceType = newType }
            in
                ( (Debug.log "ChangeType" newmodel), Cmd.none )

        FetchFail error ->
            let
                err =
                    toString error
            in
                Debug.log err ( { model | error = err }, Cmd.none )

        FetchSucceeded entries ->
            Debug.log (toString entries)
                ( { model
                    | entries = entries
                    , currentModel = lstGet 0 entries
                    , error = ""
                    , status = (List.length entries |> toString) ++ " versions found"
                    , log =
                        case entries of
                            [] ->
                                model.log

                            _ ->
                                Hv.add
                                    { resId = model.resourceId
                                    , resType = model.resourceType
                                    , resTypeLabel = resourceLabel model.resourceType
                                    }
                                    model.log
                  }
                , Cmd.none
                )

        ChangeVersion strIndex ->
            let
                idx =
                    strIndex |> String.toInt |> Result.withDefault 0
            in
                ( { model | currentModel = lstGet idx model.entries }, Cmd.none )

        KeyPress key ->
            if key == 13 then
                {- 13 is enter / carriage return -}
                updateGetAction model
            else
                updateNoOp model

        HistoryMsg hmsg ->
            case hmsg of
                Hv.Clicked logEntry ->
                    { model
                        | resourceType = logEntry.resType
                        , resourceId = logEntry.resId
                    }
                        |> updateGetAction


updateNoOp : Model -> ( Model, Cmd Msg )
updateNoOp model =
    ( model, Cmd.none )


updateGetAction : Model -> ( Model, Cmd Msg )
updateGetAction model =
    let
        newmodel =
            { model | error = "", status = "" }
    in
        ( (Debug.log "GetAction" newmodel), getData newmodel.resourceType newmodel.resourceId )



-- VIEW


{-| on change event so that FF works - added on trunk of elm but for now have this hack here
-}
onChange : (String -> Msg) -> Html.Attribute Msg
onChange tagger =
    on "change" (Json.map tagger targetValue)


onKeyPress : (Int -> Msg) -> Html.Attribute Msg
onKeyPress tagger =
    on "keypress" (Json.map tagger keyCode)


view : Model -> Html Msg
view model =
    let
        resview =
            case model.currentModel of
                Nothing ->
                    []

                Just entry ->
                    [ Entry.render entry ]

        reslst =
            case model.entries of
                [] ->
                    if model.status == "" then
                        []
                    else
                        [ renderOption ( "", "No results for " ++ model.resourceType ++ "/" ++ model.resourceId ) ]

                _ ->
                    renderResLst model.entries
    in
        div [ id "resapp" ]
            [ div [ id "resform" ]
                [ label [ for "restype" ] [ text "Resource: " ]
                , resourceList |> List.map renderOption |> select [ id "restype", onChange ChangeType ]
                , label [ for "resid" ] [ text "ID: " ]
                , input [ type' "text", id "resid", value model.resourceId, onKeyPress KeyPress, onInput Entry ] []
                , input [ type' "button", value "Get", onClick GetAction ] []
                , label [ for "reslst" ] [ text "Results: " ]
                , select [ id "reslst", onChange ChangeVersion ] reslst
                ]
            , div [ id "resview" ] resview
            , App.map HistoryMsg (Hv.view model.log)
            ]


renderResLst : List Entry.Model -> List (Html a)
renderResLst entries =
    entries |> toSelectTuples [] |> List.map renderOption


renderOption : ( String, String ) -> Html a
renderOption ( val, label ) =
    option [ value val ] [ text label ]



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- LOGIC


type alias Resource =
    ( String, String )


resourceList : List Resource
resourceList =
    [ ( "departures", "Departures" )
    , ( "tour_dossiers", "Tour Dossiers" )
    , ( "promotions", "Promotions" )
    ]


resourceLabel : String -> String
resourceLabel rType =
    resourceList
        |> List.filter (\x -> fst (x) == rType)
        |> List.head
        |> Maybe.withDefault ( "", "" )
        |> snd


getData : String -> String -> Cmd Msg
getData resType resId =
    let
        url =
            "http://res-log.appspot.com/l/" ++ resType ++ "/" ++ resId
    in
        Task.perform FetchFail FetchSucceeded (Http.get decodeData url)


decodeData : Json.Decoder (List Entry.Model)
decodeData =
    Json.list Entry.decode


toSelectTuples : List ( String, String ) -> List Entry.Model -> List ( String, String )
toSelectTuples acc xs =
    case xs of
        [] ->
            List.reverse acc

        hd :: tl ->
            let
                newacc =
                    ( List.length acc |> toString, hd.fetchdate ) :: acc
            in
                toSelectTuples newacc tl


{-|
helper func to retrieve item from list using index
-}
lstGet : Int -> List a -> Maybe a
lstGet index list =
    list |> List.drop index |> List.head
